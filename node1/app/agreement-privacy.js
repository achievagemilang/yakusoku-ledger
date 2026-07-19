'use strict';

var crypto = require('crypto');

function normalizeRequired(value, field) {
	var normalized = String(value || '').trim().toLowerCase();
	if (!normalized) {
		throw new Error(field + ' is required for private agreement details');
	}
	return normalized;
}

function createPrivateDetails(studentName, email, randomBytes) {
	var generateBytes = randomBytes || crypto.randomBytes;
	var salt = generateBytes(32);
	if (!Buffer.isBuffer(salt) || salt.length !== 32) {
		throw new Error('Privacy salt generator must return exactly 32 bytes');
	}
	return {
		StudentName: normalizeRequired(studentName, 'studentName'),
		Email: normalizeRequired(email, 'email'),
		Salt: salt.toString('hex')
	};
}

function createTransientMap(studentName, email, randomBytes) {
	var details = createPrivateDetails(studentName, email, randomBytes);
	return {
		agreement_pii: Buffer.from(JSON.stringify(details), 'utf8')
	};
}

function createEmailVerificationTransientMap(email) {
	return {
		student_email: Buffer.from(normalizeRequired(email, 'email'), 'utf8')
	};
}

exports.createPrivateDetails = createPrivateDetails;
exports.createTransientMap = createTransientMap;
exports.createEmailVerificationTransientMap = createEmailVerificationTransientMap;
