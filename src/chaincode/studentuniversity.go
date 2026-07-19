package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/lib/cid"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
)

const (
	statusSubmitted = "submitted"
	statusApproved  = "approved"
	statusRejected  = "rejected"
	maxSafeMinor    = int64(9007199254740991)
	privacyVersion  = 1

	agreementPIICollection = "agreementPIICollection"
	agreementPIITransient  = "agreement_pii"
	studentEmailTransient  = "student_email"
)

var agreementReferencePattern = regexp.MustCompile(`^AGR-[0-9]{4}-[A-F0-9]{12}$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var supportedCurrencies = map[string]bool{
	"AUD": true,
	"CAD": true,
	"EUR": true,
	"GBP": true,
	"INR": true,
	"JPY": true,
	"KRW": true,
	"USD": true,
}

type StudentUniversityContract struct{}

type agreement struct {
	ContractID        string      `json:"Key"`
	StudentName       string      `json:"StudentName,omitempty"`
	UniversityName    string      `json:"UniversityName"`
	Date              string      `json:"Date"`
	AmountMinor       int64       `json:"AmountMinor,omitempty"`
	Currency          string      `json:"Currency,omitempty"`
	LegacyAmount      json.Number `json:"Amount,omitempty"`
	Email             string      `json:"Email,omitempty"`
	StudentCommitment string      `json:"StudentCommitment,omitempty"`
	PrivacyVersion    int         `json:"PrivacyVersion,omitempty"`
	DocumentHash      string      `json:"DocumentHash"`
	Status            string      `json:"Status"`
	CreatedBy         string      `json:"CreatedBy"`
	ReviewedBy        string      `json:"ReviewedBy,omitempty"`
	UpdatedAt         string      `json:"UpdatedAt"`
}

type privateAgreementDetails struct {
	StudentName string `json:"StudentName"`
	Email       string `json:"Email"`
	Salt        string `json:"Salt"`
}

type queryRecord struct {
	Key   string          `json:"Key"`
	Value json.RawMessage `json:"Value"`
}

type historyRecord struct {
	TxID      string          `json:"TxId"`
	Value     json.RawMessage `json:"Value"`
	Timestamp string          `json:"Timestamp"`
	IsDelete  bool            `json:"IsDelete"`
}

func (t *StudentUniversityContract) Init(stub shim.ChaincodeStubInterface) peer.Response {
	_, args := stub.GetFunctionAndParameters()
	if len(args) != 0 {
		return shim.Error("Init does not accept arguments")
	}
	return shim.Success(nil)
}

func (t *StudentUniversityContract) Invoke(stub shim.ChaincodeStubInterface) peer.Response {
	function, args := stub.GetFunctionAndParameters()

	switch function {
	case "initStudentUniversity", "createAgreement":
		return t.createAgreement(stub, args)
	case "migrateAgreementPII":
		return t.migrateAgreementPII(stub, args)
	case "verifyStudentIdentity":
		return t.verifyStudentIdentity(stub, args)
	case "queryByStudentEmail":
		return t.queryByStudentEmail(stub, args)
	case "queryByStudentName":
		return t.queryByStudentName(stub, args)
	case "queryByUniversityName":
		return t.queryByUniversityName(stub, args)
	case "queryAllAgreements":
		return t.queryAllAgreements(stub, args)
	case "getHistoryForStudent":
		return t.getHistoryForStudent(stub, args)
	case "getHistoryForAgreement":
		return t.getHistoryForAgreement(stub, args)
	case "invokeFunctionStudentUniversity", "getAgreement":
		return t.getAgreement(stub, args)
	case "reviewAgreement":
		return t.reviewAgreement(stub, args)
	case "verifyDocument":
		return t.verifyDocument(stub, args)
	default:
		return shim.Error("Received unknown function invocation: " + function)
	}
}

func (t *StudentUniversityContract) createAgreement(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 6 {
		return shim.Error("Incorrect number of arguments. Expecting 6 public agreement fields")
	}
	for i, arg := range args[:5] {
		if strings.TrimSpace(arg) == "" {
			return shim.Error(fmt.Sprintf("Argument %d must be a non-empty string", i+1))
		}
	}

	contractID := strings.ToUpper(strings.TrimSpace(args[0]))
	if !agreementReferencePattern.MatchString(contractID) {
		return shim.Error("1st argument must be an agreement reference like AGR-2026-12AB34CD56EF")
	}

	currentDate := strings.TrimSpace(args[1])
	if _, err := time.Parse("2006-01-02", currentDate); err != nil {
		return shim.Error("2nd argument must be an ISO date in YYYY-MM-DD format")
	}

	amountMinor, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
	if err != nil || amountMinor <= 0 || amountMinor > maxSafeMinor {
		return shim.Error("3rd argument must be a positive safe integer in minor currency units")
	}
	currency := strings.ToUpper(strings.TrimSpace(args[3]))
	if !currencyPattern.MatchString(currency) || !supportedCurrencies[currency] {
		return shim.Error("4th argument must be a supported ISO currency code")
	}
	universityName := strings.ToLower(strings.TrimSpace(args[4]))
	documentHash := strings.ToLower(strings.TrimSpace(args[5]))
	if documentHash != "" && !isSHA256(documentHash) {
		return shim.Error("6th argument must be an empty value or a SHA-256 hash")
	}

	creatorMSP, err := cid.GetMSPID(stub)
	if err != nil {
		return shim.Error("Unable to identify transaction creator: " + err.Error())
	}
	if creatorMSP != "StudentMSP" {
		return shim.Error("Only a StudentMSP member may submit agreements")
	}
	privateDetails, err := privateDetailsFromTransient(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	updatedAt, err := transactionTime(stub)
	if err != nil {
		return shim.Error(err.Error())
	}

	existing, err := stub.GetState(contractID)
	if err != nil {
		return shim.Error(err.Error())
	}
	if existing != nil {
		return shim.Error("Agreement reference already exists: " + contractID)
	}

	record := agreement{
		ContractID:        contractID,
		UniversityName:    universityName,
		Date:              currentDate,
		AmountMinor:       amountMinor,
		Currency:          currency,
		StudentCommitment: studentCommitment(privateDetails.Email, privateDetails.Salt),
		PrivacyVersion:    privacyVersion,
		DocumentHash:      documentHash,
		Status:            statusSubmitted,
		CreatedBy:         creatorMSP,
		UpdatedAt:         updatedAt,
	}
	privatePayload, err := json.Marshal(privateDetails)
	if err != nil {
		return shim.Error(err.Error())
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutPrivateData(agreementPIICollection, contractID, privatePayload); err != nil {
		return shim.Error("Unable to store private agreement details: " + err.Error())
	}
	if err = stub.PutState(contractID, payload); err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.SetEvent("AgreementSubmitted", payload); err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) migrateAgreementPII(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one agreement ID")
	}
	creatorMSP, err := cid.GetMSPID(stub)
	if err != nil {
		return shim.Error("Unable to identify transaction creator: " + err.Error())
	}
	if creatorMSP != "StudentMSP" {
		return shim.Error("Only a StudentMSP member may migrate agreement PII")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.PrivacyVersion >= privacyVersion {
		return shim.Error("Agreement PII has already been migrated")
	}
	if strings.TrimSpace(record.StudentName) == "" || strings.TrimSpace(record.Email) == "" {
		return shim.Error("Legacy agreement does not contain migratable student PII")
	}
	privateDetails, err := privateDetailsFromTransient(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if privateDetails.StudentName != strings.ToLower(strings.TrimSpace(record.StudentName)) ||
		privateDetails.Email != strings.ToLower(strings.TrimSpace(record.Email)) {
		return shim.Error("Transient PII does not match the legacy agreement")
	}

	privatePayload, err := json.Marshal(privateDetails)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutPrivateData(agreementPIICollection, record.ContractID, privatePayload); err != nil {
		return shim.Error("Unable to store private agreement details: " + err.Error())
	}
	record.StudentCommitment = studentCommitment(privateDetails.Email, privateDetails.Salt)
	record.PrivacyVersion = privacyVersion
	record.StudentName = ""
	record.Email = ""
	record.UpdatedAt, err = transactionTime(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutState(record.ContractID, payload); err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.SetEvent("AgreementPIIMigrated", payload); err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) verifyStudentIdentity(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one agreement ID")
	}
	if err := requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.PrivacyVersion < privacyVersion {
		return shim.Error("Legacy agreement PII must be migrated before private verification")
	}
	details, err := loadPrivateDetails(stub, record.ContractID)
	if err != nil {
		return shim.Error(err.Error())
	}
	transient, err := stub.GetTransient()
	if err != nil {
		return shim.Error("Unable to read transient data: " + err.Error())
	}
	emailBytes, ok := transient[studentEmailTransient]
	if !ok || strings.TrimSpace(string(emailBytes)) == "" {
		return shim.Error("Transient student_email is required")
	}
	candidate := studentCommitment(strings.ToLower(strings.TrimSpace(string(emailBytes))), details.Salt)
	verified := subtle.ConstantTimeCompare([]byte(candidate), []byte(record.StudentCommitment)) == 1
	payload, err := json.Marshal(map[string]interface{}{
		"agreementId": record.ContractID,
		"verified":    verified,
	})
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) reviewAgreement(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting agreement ID and decision")
	}
	decision := strings.ToLower(strings.TrimSpace(args[1]))
	if decision != statusApproved && decision != statusRejected {
		return shim.Error("Decision must be approved or rejected")
	}

	reviewerMSP, err := cid.GetMSPID(stub)
	if err != nil {
		return shim.Error("Unable to identify reviewer: " + err.Error())
	}
	if reviewerMSP != "UniversityMSP" {
		return shim.Error("Only a UniversityMSP member may review agreements")
	}

	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.PrivacyVersion < privacyVersion &&
		(strings.TrimSpace(record.StudentName) != "" || strings.TrimSpace(record.Email) != "") {
		return shim.Error("Legacy agreement PII must be migrated before review")
	}
	if record.Status != "" && record.Status != statusSubmitted {
		return shim.Error("Only submitted agreements may be reviewed")
	}
	record.Status = decision
	record.ReviewedBy = reviewerMSP
	record.UpdatedAt, err = transactionTime(stub)
	if err != nil {
		return shim.Error(err.Error())
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutState(record.ContractID, payload); err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.SetEvent("AgreementReviewed", payload); err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) verifyDocument(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting agreement ID and SHA-256 hash")
	}
	hash := strings.ToLower(strings.TrimSpace(args[1]))
	if !isSHA256(hash) {
		return shim.Error("Document hash must be a SHA-256 hash")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	result := map[string]interface{}{
		"agreementId": record.ContractID,
		"verified":    record.DocumentHash != "" && record.DocumentHash == hash,
		"status":      record.Status,
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) queryByStudentEmail(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	return t.queryByPrivateField(stub, args, "Email", "email")
}

func (t *StudentUniversityContract) queryByStudentName(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	return t.queryByPrivateField(stub, args, "StudentName", "student name")
}

func (t *StudentUniversityContract) queryByUniversityName(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	return t.queryByField(stub, args, "UniversityName", "university name")
}

func (t *StudentUniversityContract) queryByField(stub shim.ChaincodeStubInterface, args []string, field, description string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one " + description)
	}
	selector := map[string]interface{}{
		"selector": map[string]string{field: strings.ToLower(strings.TrimSpace(args[0]))},
	}
	queryBytes, err := json.Marshal(selector)
	if err != nil {
		return shim.Error(err.Error())
	}
	results, err := queryByString(stub, string(queryBytes))
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(results)
}

func (t *StudentUniversityContract) queryByPrivateField(stub shim.ChaincodeStubInterface, args []string, field, description string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one " + description)
	}
	if err := requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	expected := strings.ToLower(strings.TrimSpace(args[0]))
	iterator, err := stub.GetStateByRange("", "")
	if err != nil {
		return shim.Error(err.Error())
	}
	defer iterator.Close()

	records := make([]queryRecord, 0)
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		record, err := agreementWithPrivateData(stub, result.Value)
		if err != nil {
			return shim.Error(err.Error())
		}
		value := record.StudentName
		if field == "Email" {
			value = record.Email
		}
		if strings.ToLower(strings.TrimSpace(value)) != expected {
			continue
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return shim.Error(err.Error())
		}
		records = append(records, queryRecord{Key: result.Key, Value: json.RawMessage(payload)})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) queryAllAgreements(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 0 {
		return shim.Error("queryAllAgreements does not accept arguments")
	}
	if err := requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	iterator, err := stub.GetStateByRange("", "")
	if err != nil {
		return shim.Error(err.Error())
	}
	defer iterator.Close()

	records := make([]queryRecord, 0)
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		record, err := agreementWithPrivateData(stub, result.Value)
		if err != nil {
			return shim.Error(err.Error())
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return shim.Error(err.Error())
		}
		records = append(records, queryRecord{Key: result.Key, Value: json.RawMessage(payload)})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) getAgreement(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one agreement ID")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	if record.PrivacyVersion >= privacyVersion {
		record, err = hydrateAgreement(stub, record)
		if err != nil {
			return shim.Error(err.Error())
		}
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) getHistoryForStudent(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting student and university names")
	}
	contractID := agreementID(
		strings.ToLower(strings.TrimSpace(args[0])),
		strings.ToLower(strings.TrimSpace(args[1])),
	)
	return agreementHistory(stub, contractID)
}

func (t *StudentUniversityContract) getHistoryForAgreement(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one agreement ID")
	}
	return agreementHistory(stub, strings.TrimSpace(args[0]))
}

func agreementHistory(stub shim.ChaincodeStubInterface, contractID string) peer.Response {
	if err := requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	iterator, err := stub.GetHistoryForKey(contractID)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer iterator.Close()

	records := make([]historyRecord, 0)
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		value := json.RawMessage("null")
		if !result.IsDelete {
			value = json.RawMessage(result.Value)
		}
		records = append(records, historyRecord{
			TxID:      result.TxId,
			Value:     value,
			Timestamp: time.Unix(result.Timestamp.Seconds, int64(result.Timestamp.Nanos)).UTC().Format(time.RFC3339),
			IsDelete:  result.IsDelete,
		})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func queryByString(stub shim.ChaincodeStubInterface, query string) ([]byte, error) {
	iterator, err := stub.GetQueryResult(query)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	records := make([]queryRecord, 0)
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		records = append(records, queryRecord{Key: result.Key, Value: json.RawMessage(result.Value)})
	}
	return json.Marshal(records)
}

func loadAgreement(stub shim.ChaincodeStubInterface, contractID string) (*agreement, error) {
	payload, err := stub.GetState(contractID)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("agreement %s does not exist", contractID)
	}
	record := &agreement{}
	if err = json.Unmarshal(payload, record); err != nil {
		return nil, err
	}
	return record, nil
}

func agreementWithPrivateData(stub shim.ChaincodeStubInterface, payload []byte) (*agreement, error) {
	record := &agreement{}
	if err := json.Unmarshal(payload, record); err != nil {
		return nil, err
	}
	if record.PrivacyVersion < privacyVersion {
		return record, nil
	}
	return hydrateAgreement(stub, record)
}

func hydrateAgreement(stub shim.ChaincodeStubInterface, record *agreement) (*agreement, error) {
	payload, err := stub.GetPrivateData(agreementPIICollection, record.ContractID)
	if err != nil {
		return nil, fmt.Errorf("unable to read private agreement details: %s", err)
	}
	if payload == nil {
		return record, nil
	}
	details := &privateAgreementDetails{}
	if err = json.Unmarshal(payload, details); err != nil {
		return nil, fmt.Errorf("invalid private agreement details for %s: %s", record.ContractID, err)
	}
	record.StudentName = details.StudentName
	record.Email = details.Email
	return record, nil
}

func loadPrivateDetails(stub shim.ChaincodeStubInterface, contractID string) (*privateAgreementDetails, error) {
	payload, err := stub.GetPrivateData(agreementPIICollection, contractID)
	if err != nil {
		return nil, fmt.Errorf("unable to read private agreement details: %s", err)
	}
	if payload == nil {
		return nil, fmt.Errorf("private agreement details are unavailable for %s", contractID)
	}
	details := &privateAgreementDetails{}
	if err = json.Unmarshal(payload, details); err != nil {
		return nil, fmt.Errorf("invalid private agreement details for %s: %s", contractID, err)
	}
	return details, nil
}

func privateDetailsFromTransient(stub shim.ChaincodeStubInterface) (*privateAgreementDetails, error) {
	transient, err := stub.GetTransient()
	if err != nil {
		return nil, fmt.Errorf("unable to read transient data: %s", err)
	}
	payload, ok := transient[agreementPIITransient]
	if !ok || len(payload) == 0 {
		return nil, fmt.Errorf("transient agreement_pii is required")
	}
	details := &privateAgreementDetails{}
	if err = json.Unmarshal(payload, details); err != nil {
		return nil, fmt.Errorf("transient agreement_pii must be valid JSON: %s", err)
	}
	details.StudentName = strings.ToLower(strings.TrimSpace(details.StudentName))
	details.Email = strings.ToLower(strings.TrimSpace(details.Email))
	details.Salt = strings.ToLower(strings.TrimSpace(details.Salt))
	if details.StudentName == "" || details.Email == "" {
		return nil, fmt.Errorf("private student name and email must be non-empty strings")
	}
	if !isSHA256(details.Salt) {
		return nil, fmt.Errorf("private salt must be 32 random bytes encoded as hexadecimal")
	}
	return details, nil
}

func requirePIIReader(stub shim.ChaincodeStubInterface) error {
	mspID, err := cid.GetMSPID(stub)
	if err != nil {
		return fmt.Errorf("unable to identify transaction creator: %s", err)
	}
	if mspID != "StudentMSP" && mspID != "UniversityMSP" {
		return fmt.Errorf("organization %s is not authorized to read agreement PII", mspID)
	}
	return nil
}

func studentCommitment(email, salt string) string {
	hash := sha256.Sum256([]byte(salt + ":" + strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(hash[:])
}

func agreementID(studentName, universityName string) string {
	hash := sha256.Sum256([]byte(studentName + universityName))
	return hex.EncodeToString(hash[:])
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func transactionTime(stub shim.ChaincodeStubInterface) (string, error) {
	timestamp, err := stub.GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("unable to read transaction timestamp: %s", err)
	}
	var buffer bytes.Buffer
	buffer.WriteString(time.Unix(timestamp.Seconds, int64(timestamp.Nanos)).UTC().Format(time.RFC3339))
	return buffer.String(), nil
}

func main() {
	if err := shim.Start(new(StudentUniversityContract)); err != nil {
		fmt.Printf("Error starting Yakusoku chaincode: %s", err)
	}
}
