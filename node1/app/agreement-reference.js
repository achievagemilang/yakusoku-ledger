'use strict';

var crypto = require('crypto');

var agreementReferencePattern = /^AGR-[0-9]{4}-[A-F0-9]{12}$/;

function createAgreementReference(date) {
	var year = (date || new Date()).getUTCFullYear();
	return 'AGR-' + year + '-' + crypto.randomBytes(6).toString('hex').toUpperCase();
}

function isAgreementReference(value) {
	return agreementReferencePattern.test(String(value || ''));
}

exports.createAgreementReference = createAgreementReference;
exports.isAgreementReference = isAgreementReference;
