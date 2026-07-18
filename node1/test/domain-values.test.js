'use strict';

var assert = require('assert');
var references = require('../app/agreement-reference.js');
var money = require('../app/money.js');

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

console.log('Domain value tests passed');
