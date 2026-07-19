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
	statusDraft             = "draft"
	statusPendingUniversity = "pending_university"
	statusActive            = "active"
	statusAmendmentProposed = "amendment_proposed"
	statusRejected          = "rejected"
	statusExpired           = "expired"
	statusSubmitted         = "submitted"
	statusApproved          = "approved"
	maxSafeMinor            = int64(9007199254740991)
	privacyVersion          = 1

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
	ExpiresOn        string      `json:"ExpiresOn,omitempty"`
	AmountMinor      int64       `json:"AmountMinor,omitempty"`
	Currency          string      `json:"Currency,omitempty"`
	LegacyAmount      json.Number `json:"Amount,omitempty"`
	Email             string      `json:"Email,omitempty"`
	StudentCommitment string      `json:"StudentCommitment,omitempty"`
	PrivacyVersion    int         `json:"PrivacyVersion,omitempty"`
	DocumentHash      string      `json:"DocumentHash"`
	Status            string      `json:"Status"`
	Revision          int         `json:"Revision,omitempty"`
	CreatedBy         string      `json:"CreatedBy"`
	StudentSignedBy   string      `json:"StudentSignedBy,omitempty"`
	StudentSignedAt   string      `json:"StudentSignedAt,omitempty"`
	UniversitySignedBy string     `json:"UniversitySignedBy,omitempty"`
	UniversitySignedAt string     `json:"UniversitySignedAt,omitempty"`
	ReviewedBy       string      `json:"ReviewedBy,omitempty"`
	PendingAmendment *amendment  `json:"PendingAmendment,omitempty"`
	Amendments       []amendment `json:"Amendments,omitempty"`
	ExpiredBy        string      `json:"ExpiredBy,omitempty"`
	ExpiredAt        string      `json:"ExpiredAt,omitempty"`
	UpdatedAt         string      `json:"UpdatedAt"`
}

type amendment struct {
	ID                   string `json:"Id"`
	Date                 string `json:"Date"`
	ExpiresOn            string `json:"ExpiresOn,omitempty"`
	AmountMinor          int64  `json:"AmountMinor"`
	Currency             string `json:"Currency"`
	DocumentHash         string `json:"DocumentHash"`
	ProposedBy            string `json:"ProposedBy"`
	ProposedByMSP         string `json:"ProposedByMSP"`
	ProposedAt            string `json:"ProposedAt"`
	StudentApprovedBy     string `json:"StudentApprovedBy,omitempty"`
	StudentApprovedAt     string `json:"StudentApprovedAt,omitempty"`
	UniversityApprovedBy  string `json:"UniversityApprovedBy,omitempty"`
	UniversityApprovedAt  string `json:"UniversityApprovedAt,omitempty"`
	State                 string `json:"State,omitempty"`
	SupersededRevision    int    `json:"SupersededRevision,omitempty"`
	ActivatedRevision     int    `json:"ActivatedRevision,omitempty"`
	ResolvedAt            string `json:"ResolvedAt,omitempty"`
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
	Key       string          `json:"Key,omitempty"`
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
	case "signAgreement":
		return t.signAgreement(stub, args)
	case "proposeAmendment":
		return t.proposeAmendment(stub, args)
	case "decideAmendment":
		return t.decideAmendment(stub, args)
	case "expireAgreement":
		return t.expireAgreement(stub, args)
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
	case "getHistoryForStudentLegacy":
		return t.getHistoryForStudentLegacy(stub, args)
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
	if len(args) != 7 {
		return shim.Error("Incorrect number of arguments. Expecting 7 public agreement fields")
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
	expiresOn := strings.TrimSpace(args[2])
	if _, err := time.Parse("2006-01-02", expiresOn); err != nil {
		return shim.Error("3rd argument must be an ISO expiration date in YYYY-MM-DD format")
	}
	if expiresOn < currentDate {
		return shim.Error("Expiration date must not be before the effective date")
	}

	amountMinor, err := strconv.ParseInt(strings.TrimSpace(args[3]), 10, 64)
	if err != nil || amountMinor <= 0 || amountMinor > maxSafeMinor {
		return shim.Error("4th argument must be a positive safe integer in minor currency units")
	}
	currency := strings.ToUpper(strings.TrimSpace(args[4]))
	if !currencyPattern.MatchString(currency) || !supportedCurrencies[currency] {
		return shim.Error("5th argument must be a supported ISO currency code")
	}
	universityName := strings.ToLower(strings.TrimSpace(args[5]))
	documentHash := strings.ToLower(strings.TrimSpace(args[6]))
	if documentHash != "" && !isSHA256(documentHash) {
		return shim.Error("7th argument must be an empty value or a SHA-256 hash")
	}

	creatorMSP, err := cid.GetMSPID(stub)
	if err != nil {
		return shim.Error("Unable to identify transaction creator: " + err.Error())
	}
	if creatorMSP != "StudentMSP" {
		return shim.Error("Only a StudentMSP member may submit agreements")
	}
	if err = requireOrganizationRole(stub, creatorMSP); err != nil {
		return shim.Error(err.Error())
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
		ExpiresOn:         expiresOn,
		AmountMinor:       amountMinor,
		Currency:          currency,
		StudentCommitment: studentCommitment(privateDetails.Email, privateDetails.Salt),
		PrivacyVersion:    privacyVersion,
		DocumentHash:      documentHash,
		Status:            statusDraft,
		Revision:          1,
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
	if err = requireOrganizationRole(stub, creatorMSP); err != nil {
		return shim.Error(err.Error())
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

func (t *StudentUniversityContract) signAgreement(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one agreement ID")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.PrivacyVersion < privacyVersion &&
		(strings.TrimSpace(record.StudentName) != "" || strings.TrimSpace(record.Email) != "") {
		return shim.Error("Legacy agreement PII must be migrated before signing")
	}
	mspID, signer, signedAt, err := signingIdentity(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = requireOrganizationRole(stub, mspID); err != nil {
		return shim.Error(err.Error())
	}

	switch record.Status {
	case statusDraft:
		if mspID != "StudentMSP" {
			return shim.Error("Only a StudentMSP member may sign a draft agreement")
		}
		record.StudentSignedBy = signer
		record.StudentSignedAt = signedAt
		record.Status = statusPendingUniversity
	case statusPendingUniversity, statusSubmitted:
		if mspID != "UniversityMSP" {
			return shim.Error("Only a UniversityMSP member may countersign a student-signed agreement")
		}
		if record.Status == statusSubmitted && record.StudentSignedBy == "" {
			record.StudentSignedBy = "StudentMSP:legacy-submission"
			record.StudentSignedAt = record.UpdatedAt
		}
		if record.StudentSignedBy == "" {
			return shim.Error("Student signature is required before university countersignature")
		}
		record.UniversitySignedBy = signer
		record.UniversitySignedAt = signedAt
		record.ReviewedBy = signer
		record.Status = statusActive
	default:
		return shim.Error("Agreement cannot be signed while in state " + record.Status)
	}
	record.UpdatedAt = signedAt
	return saveAgreement(stub, record, "AgreementSigned")
}

func (t *StudentUniversityContract) proposeAmendment(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 6 {
		return shim.Error("Incorrect number of arguments. Expecting agreement ID and five amendment fields")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.Status != statusActive && record.Status != statusApproved {
		return shim.Error("Only an active agreement may be amended")
	}
	mspID, proposer, proposedAt, err := signingIdentity(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if mspID != "StudentMSP" && mspID != "UniversityMSP" {
		return shim.Error("Only agreement member organizations may propose amendments")
	}
	if err = requireOrganizationRole(stub, mspID); err != nil {
		return shim.Error(err.Error())
	}
	effectiveDate := strings.TrimSpace(args[1])
	expiresOn := strings.TrimSpace(args[2])
	if _, err = time.Parse("2006-01-02", effectiveDate); err != nil {
		return shim.Error("Amendment effective date must use YYYY-MM-DD")
	}
	if _, err = time.Parse("2006-01-02", expiresOn); err != nil {
		return shim.Error("Amendment expiration date must use YYYY-MM-DD")
	}
	if expiresOn < effectiveDate {
		return shim.Error("Amendment expiration date must not precede its effective date")
	}
	amountMinor, err := strconv.ParseInt(strings.TrimSpace(args[3]), 10, 64)
	if err != nil || amountMinor <= 0 || amountMinor > maxSafeMinor {
		return shim.Error("Amendment amount must be a positive safe integer in minor currency units")
	}
	currency := strings.ToUpper(strings.TrimSpace(args[4]))
	if !currencyPattern.MatchString(currency) || !supportedCurrencies[currency] {
		return shim.Error("Amendment currency must be a supported ISO currency code")
	}
	documentHash := strings.ToLower(strings.TrimSpace(args[5]))
	if !isSHA256(documentHash) {
		return shim.Error("Amendment document hash must be a SHA-256 hash")
	}
	pending := &amendment{
		ID:            amendmentID(stub.GetTxID()),
		Date:          effectiveDate,
		ExpiresOn:     expiresOn,
		AmountMinor:   amountMinor,
		Currency:      currency,
		DocumentHash:  documentHash,
		ProposedBy:     proposer,
		ProposedByMSP:  mspID,
		ProposedAt:     proposedAt,
	}
	if mspID == "StudentMSP" {
		pending.StudentApprovedBy = proposer
		pending.StudentApprovedAt = proposedAt
	} else {
		pending.UniversityApprovedBy = proposer
		pending.UniversityApprovedAt = proposedAt
	}
	record.PendingAmendment = pending
	record.Status = statusAmendmentProposed
	record.UpdatedAt = proposedAt
	return saveAgreement(stub, record, "AmendmentProposed")
}

func (t *StudentUniversityContract) decideAmendment(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting agreement ID and decision")
	}
	decision := strings.ToLower(strings.TrimSpace(args[1]))
	if decision != "approved" && decision != "rejected" {
		return shim.Error("Amendment decision must be approved or rejected")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.Status != statusAmendmentProposed || record.PendingAmendment == nil {
		return shim.Error("Agreement has no pending amendment")
	}
	mspID, signer, resolvedAt, err := signingIdentity(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if mspID != "StudentMSP" && mspID != "UniversityMSP" {
		return shim.Error("Only agreement member organizations may decide amendments")
	}
	if err = requireOrganizationRole(stub, mspID); err != nil {
		return shim.Error(err.Error())
	}
	pending := record.PendingAmendment
	if decision == "rejected" {
		pending.State = statusRejected
		pending.ResolvedAt = resolvedAt
		record.Amendments = append(record.Amendments, *pending)
		record.PendingAmendment = nil
		record.Status = statusActive
		record.UpdatedAt = resolvedAt
		return saveAgreement(stub, record, "AmendmentRejected")
	}
	if mspID == "StudentMSP" {
		if pending.StudentApprovedBy != "" {
			return shim.Error("Student organization has already approved this amendment")
		}
		pending.StudentApprovedBy = signer
		pending.StudentApprovedAt = resolvedAt
	} else {
		if pending.UniversityApprovedBy != "" {
			return shim.Error("University organization has already approved this amendment")
		}
		pending.UniversityApprovedBy = signer
		pending.UniversityApprovedAt = resolvedAt
	}
	if pending.StudentApprovedBy == "" || pending.UniversityApprovedBy == "" {
		record.UpdatedAt = resolvedAt
		return saveAgreement(stub, record, "AmendmentApproved")
	}

	if record.Revision == 0 {
		record.Revision = 1
	}
	pending.State = "applied"
	pending.SupersededRevision = record.Revision
	pending.ActivatedRevision = record.Revision + 1
	pending.ResolvedAt = resolvedAt
	record.Date = pending.Date
	record.ExpiresOn = pending.ExpiresOn
	record.AmountMinor = pending.AmountMinor
	record.Currency = pending.Currency
	record.DocumentHash = pending.DocumentHash
	record.Revision = pending.ActivatedRevision
	record.Amendments = append(record.Amendments, *pending)
	record.PendingAmendment = nil
	record.Status = statusActive
	record.UpdatedAt = resolvedAt
	return saveAgreement(stub, record, "AmendmentApplied")
}

func (t *StudentUniversityContract) expireAgreement(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return shim.Error("Incorrect number of arguments. Expecting one agreement ID")
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.Status != statusActive && record.Status != statusApproved {
		return shim.Error("Only an active agreement may expire")
	}
	if record.ExpiresOn == "" {
		return shim.Error("Agreement has no expiration date")
	}
	mspID, signer, expiredAt, err := signingIdentity(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	if mspID != "StudentMSP" && mspID != "UniversityMSP" {
		return shim.Error("Only agreement member organizations may record expiration")
	}
	if err = requireOrganizationRole(stub, mspID); err != nil {
		return shim.Error(err.Error())
	}
	if expiredAt[:10] < record.ExpiresOn {
		return shim.Error("Agreement cannot expire before " + record.ExpiresOn)
	}
	record.Status = statusExpired
	record.ExpiredBy = signer
	record.ExpiredAt = expiredAt
	record.UpdatedAt = expiredAt
	return saveAgreement(stub, record, "AgreementExpired")
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
	if err = requireOrganizationRole(stub, reviewerMSP); err != nil {
		return shim.Error(err.Error())
	}

	if decision == statusApproved {
		return t.signAgreement(stub, []string{args[0]})
	}
	record, err := loadAgreement(stub, strings.TrimSpace(args[0]))
	if err != nil {
		return shim.Error(err.Error())
	}
	if record.PrivacyVersion < privacyVersion &&
		(strings.TrimSpace(record.StudentName) != "" || strings.TrimSpace(record.Email) != "") {
		return shim.Error("Legacy agreement PII must be migrated before review")
	}
	if record.Status != statusPendingUniversity && record.Status != statusSubmitted {
		return shim.Error("Only student-signed agreements may be rejected")
	}
	record.Status = decision
	_, reviewer, reviewedAt, err := signingIdentity(stub)
	if err != nil {
		return shim.Error(err.Error())
	}
	record.ReviewedBy = reviewer
	record.UpdatedAt = reviewedAt

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

func findMatchingAgreements(stub shim.ChaincodeStubInterface, studentName, universityName string) ([]string, error) {
	selector := map[string]interface{}{
		"selector": map[string]string{"UniversityName": universityName},
	}
	queryBytes, err := json.Marshal(selector)
	if err != nil {
		return nil, err
	}
	iterator, err := stub.GetQueryResult(string(queryBytes))
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	var keys []string
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		record, err := agreementWithPrivateData(stub, result.Value)
		if err != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(record.StudentName)) != studentName {
			continue
		}
		keys = append(keys, result.Key)
	}
	return keys, nil
}

func (t *StudentUniversityContract) getHistoryForStudent(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting student and university names")
	}
	if err := requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	keys, err := findMatchingAgreements(stub,
		strings.ToLower(strings.TrimSpace(args[0])),
		strings.ToLower(strings.TrimSpace(args[1])),
	)
	if err != nil {
		return shim.Error(err.Error())
	}

	entries := make([]historyRecord, 0)
	for _, key := range keys {
		records, err := getHistoryForKey(stub, key)
		if err != nil {
			continue
		}
		for _, r := range records {
			r.Key = key
			entries = append(entries, r)
		}
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
}

func (t *StudentUniversityContract) getHistoryForStudentLegacy(stub shim.ChaincodeStubInterface, args []string) peer.Response {
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

func getHistoryForKey(stub shim.ChaincodeStubInterface, key string) ([]historyRecord, error) {
	iterator, err := stub.GetHistoryForKey(key)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	records := make([]historyRecord, 0)
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return nil, err
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
	return records, nil
}

func agreementHistory(stub shim.ChaincodeStubInterface, contractID string) peer.Response {
	if err := requirePIIReader(stub); err != nil {
		return shim.Error(err.Error())
	}
	records, err := getHistoryForKey(stub, contractID)
	if err != nil {
		return shim.Error(err.Error())
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
	return requireOrganizationRole(stub, mspID)
}

func requireOrganizationRole(stub shim.ChaincodeStubInterface, mspID string) error {
	role, found, err := cid.GetAttributeValue(stub, "yakusoku.role")
	if err != nil {
		return fmt.Errorf("unable to read yakusoku.role certificate attribute: %s", err)
	}
	if !found {
		return fmt.Errorf("identity certificate does not contain yakusoku.role")
	}
	if role == "organization_admin" {
		return nil
	}
	if mspID == "StudentMSP" && role == "student" {
		return nil
	}
	if mspID == "UniversityMSP" && role == "university_reviewer" {
		return nil
	}
	return fmt.Errorf("certificate role %s is not authorized for %s", role, mspID)
}

func signingIdentity(stub shim.ChaincodeStubInterface) (string, string, string, error) {
	mspID, err := cid.GetMSPID(stub)
	if err != nil {
		return "", "", "", fmt.Errorf("unable to identify signer organization: %s", err)
	}
	creatorID, err := cid.GetID(stub)
	if err != nil {
		return "", "", "", fmt.Errorf("unable to identify signer: %s", err)
	}
	signedAt, err := transactionTime(stub)
	if err != nil {
		return "", "", "", err
	}
	hash := sha256.Sum256([]byte(creatorID))
	fingerprint := strings.ToUpper(hex.EncodeToString(hash[:8]))
	return mspID, mspID + ":" + fingerprint, signedAt, nil
}

func amendmentID(transactionID string) string {
	normalized := strings.ToUpper(strings.TrimSpace(transactionID))
	if len(normalized) > 12 {
		normalized = normalized[:12]
	}
	return "AMD-" + normalized
}

func saveAgreement(stub shim.ChaincodeStubInterface, record *agreement, eventName string) peer.Response {
	payload, err := json.Marshal(record)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.PutState(record.ContractID, payload); err != nil {
		return shim.Error(err.Error())
	}
	if err = stub.SetEvent(eventName, payload); err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(payload)
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
