'use strict';

var assert = require('assert');
var fs = require('fs');
var path = require('path');
var references = require('../app/agreement-reference.js');
var privacy = require('../app/agreement-privacy.js');
var money = require('../app/money.js');
var governanceStore = path.join(__dirname, 'identity-governance.test.json');
process.env.IDENTITY_GOVERNANCE_STORE = governanceStore;
if (fs.existsSync(governanceStore)) fs.unlinkSync(governanceStore);
var governance = require('../app/identity-governance.js');

assert.deepStrictEqual(money.parseMoney('680000', 'JPY'), {
	amountMinor: '680000',
	currency: 'JPY'
});
assert.deepStrictEqual(money.parseMoney('12.34', 'usd'), {
	amountMinor: '1234',
	currency: 'USD'
});
assert.deepStrictEqual(money.parseMoney('0.01', 'EUR'), {
	amountMinor: '1',
	currency: 'EUR'
});
assert.throws(function() {
	money.parseMoney('1.1', 'JPY');
}, /0 decimal places/);
assert.throws(function() {
	money.parseMoney('1.234', 'USD');
}, /at most 2 decimal places/);
assert.throws(function() {
	money.parseMoney('NaN', 'USD');
}, /positive decimal string/);
assert.throws(function() {
	money.parseMoney('9007199254740992', 'JPY');
}, /supported range/);
assert.throws(function() {
	money.parseMoney('10', 'ZZZ');
}, /currency must be one of/);

var generated = new Set();
for (var i = 0; i < 1000; i++) {
	var reference = references.createAgreementReference(new Date('2026-01-01T00:00:00Z'));
	assert.strictEqual(references.isAgreementReference(reference), true);
	generated.add(reference);
}
assert.strictEqual(generated.size, 1000);
assert.strictEqual(references.isAgreementReference('invalid'), false);
assert.strictEqual(references.isAgreementReference('AGR-2026-GENESIS01'), false);

var privateDetails = privacy.createPrivateDetails(
	'Ada Lovelace',
	'ADA@EXAMPLE.COM',
	function() {
		return Buffer.alloc(32, 0xab);
	}
);
assert.deepStrictEqual(privateDetails, {
	StudentName: 'ada lovelace',
	Email: 'ada@example.com',
	Salt: new Array(33).join('ab')
});

var privateTransient = privacy.createTransientMap(
	'Ada Lovelace',
	'ADA@EXAMPLE.COM',
	function() {
		return Buffer.alloc(32, 0xab);
	}
);
assert.deepStrictEqual(
	JSON.parse(privateTransient.agreement_pii.toString('utf8')),
	privateDetails
);
assert.strictEqual(
	privacy.createEmailVerificationTransientMap(' ADA@EXAMPLE.COM ')
		.student_email.toString('utf8'),
	'ada@example.com'
);
assert.throws(function() {
	privacy.createPrivateDetails('', 'ada@example.com');
}, /studentName is required/);
assert.throws(function() {
	privacy.createPrivateDetails('Ada', '');
}, /email is required/);
assert.throws(function() {
	privacy.createPrivateDetails('Ada', 'ada@example.com', function() {
		return Buffer.alloc(16);
	});
}, /exactly 32 bytes/);

var issued = governance.createInvitation('org2', 'student', 60, 'admin');
assert.strictEqual(/^[A-Za-z0-9_-]+$/.test(issued.token), true);
assert.strictEqual(issued.invitation.status, 'issued');
var claimed = governance.claimInvitation(issued.token, 'org2', 'student-1');
assert.strictEqual(claimed.status, 'claimed');
var member = governance.completeEnrollment(claimed.id, 'student-1');
assert.strictEqual(member.role, 'student');
assert.strictEqual(member.status, 'active');
assert.throws(function() {
	governance.claimInvitation(issued.token, 'org2', 'student-2');
}, /already used/);
governance.revokeMember('org2', 'student-1', 'admin', 'test');
assert.strictEqual(governance.getMember('org2', 'student-1').status, 'revoked');
assert.strictEqual(governance.listEvents('org2').length, 4);
fs.unlinkSync(governanceStore);

console.log('Domain value tests passed');
