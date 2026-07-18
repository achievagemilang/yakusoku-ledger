'use strict';

var currencyMinorUnits = {
	AUD: 2,
	CAD: 2,
	EUR: 2,
	GBP: 2,
	INR: 2,
	JPY: 0,
	KRW: 0,
	USD: 2
};

function parseMoney(amount, currency) {
	var normalizedCurrency = String(currency || '').trim().toUpperCase();
	if (!Object.prototype.hasOwnProperty.call(currencyMinorUnits, normalizedCurrency)) {
		throw new Error('currency must be one of ' + Object.keys(currencyMinorUnits).join(', '));
	}

	var value = String(amount || '').trim();
	if (!/^\d+(?:\.\d+)?$/.test(value)) {
		throw new Error('amount must be a positive decimal string');
	}
	var parts = value.split('.');
	var scale = currencyMinorUnits[normalizedCurrency];
	var fraction = parts[1] || '';
	if (fraction.length > scale) {
		throw new Error(normalizedCurrency + ' supports at most ' + scale + ' decimal places');
	}
	var minorText = (parts[0].replace(/^0+(?=\d)/, '') || '0') +
		fraction.padEnd(scale, '0');
	var amountMinor = Number(minorText);
	if (!Number.isSafeInteger(amountMinor) || amountMinor <= 0) {
		throw new Error('amount is outside the supported range');
	}
	return {
		amountMinor: String(amountMinor),
		currency: normalizedCurrency
	};
}

exports.currencyMinorUnits = currencyMinorUnits;
exports.parseMoney = parseMoney;
