'use strict';

var crypto = require('crypto');
var http = require('http');
var util = require('util');
var bodyParser = require('body-parser');
var bearerToken = require('express-bearer-token');
var cors = require('cors');
var express = require('express');
var jwt = require('jsonwebtoken');
var log4js = require('log4js');

require('./config.js');
var hfc = require('fabric-client');

var channels = require('./app/create-channel.js');
var helper = require('./app/helper.js');
var install = require('./app/install-chaincode.js');
var instantiate = require('./app/instantiate-chaincode.js');
var invoke = require('./app/invoke-transaction.js');
var join = require('./app/join-channel.js');
var query = require('./app/query.js');

var app = express();
var logger = log4js.getLogger('YakusokuLedger');
var host = process.env.HOST || hfc.getConfigSetting('host');
var port = process.env.PORT || hfc.getConfigSetting('port');
var jwtSecret = process.env.JWT_SECRET || crypto.randomBytes(32).toString('hex');

app.set('secret', jwtSecret);
app.use(cors());
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({extended: false}));
app.use(bearerToken());

function errorResponse(field) {
	return {
		success: false,
		message: field + ' field is missing or invalid in the request'
	};
}

function requireField(res, value, field) {
	if (value === undefined || value === null || value === '' ||
		(Array.isArray(value) && value.length === 0)) {
		res.status(400).json(errorResponse('\'' + field + '\''));
		return false;
	}
	return true;
}

function parseArgs(value) {
	if (Array.isArray(value)) {
		return value;
	}
	if (typeof value !== 'string') {
		throw new Error('args must be a JSON array');
	}
	var parsed = JSON.parse(value);
	if (!Array.isArray(parsed)) {
		throw new Error('args must be a JSON array');
	}
	return parsed;
}

function sendResult(res, next, operation) {
	Promise.resolve()
		.then(operation)
		.then(function(result) {
			res.json(result);
		})
		.catch(next);
}

function sendFabricResult(res, next, orgName, operation) {
	sendResult(res, next, function() {
		return helper.runForOrg(orgName, operation);
	});
}

function requireAdmin(req, res, next) {
	if (req.role !== 'admin') {
		return res.status(403).json({
			success: false,
			message: 'Organization administrator access is required'
		});
	}
	return next();
}

app.get('/health', function(req, res) {
	res.json({status: 'ok'});
});

app.use(function(req, res, next) {
	if (req.path === '/users') {
		return next();
	}
	if (!req.token) {
		return res.status(401).json({
			success: false,
			message: 'A Bearer token from POST /users is required'
		});
	}

	jwt.verify(req.token, app.get('secret'), function(err, decoded) {
		if (err) {
			return res.status(401).json({
				success: false,
				message: 'The authentication token is invalid or expired'
			});
		}
		req.username = decoded.username;
		req.orgname = decoded.orgName;
		req.role = decoded.role;
		logger.debug(util.format(
			'Decoded JWT token for username %s in organization %s',
			decoded.username,
			decoded.orgName
		));
		return next();
	});
});

app.post('/users', function(req, res, next) {
	var username = req.body.username;
	var orgName = req.body.orgName;
	if (!requireField(res, username, 'username') ||
		!requireField(res, orgName, 'orgName')) {
		return;
	}
	if (!helper.ORGS[orgName]) {
		return res.status(400).json({
			success: false,
			message: 'orgName must be org1 or org2'
		});
	}

	var adminSecret = process.env.ADMIN_ENROLLMENT_SECRET;
	var role = adminSecret && req.body.adminSecret === adminSecret ? 'admin' : 'user';
	sendFabricResult(res, next, orgName, function() {
		return helper.getRegisteredUsers(username, orgName, true).then(function(response) {
			if (!response || typeof response === 'string') {
				throw new Error(response || 'User enrollment failed');
			}
			response.token = jwt.sign({
				exp: Math.floor(Date.now() / 1000) +
					parseInt(hfc.getConfigSetting('jwt_expiretime'), 10),
				username: username,
				orgName: orgName,
				role: role
			}, app.get('secret'));
			return response;
		});
	});
});

app.post('/channels', requireAdmin, function(req, res, next) {
	var channelName = req.body.channelName;
	var channelConfigPath = req.body.channelConfigPath;
	if (!requireField(res, channelName, 'channelName') ||
		!requireField(res, channelConfigPath, 'channelConfigPath')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return channels.createChannel(
			channelName,
			channelConfigPath,
			req.username,
			req.orgname
		);
	});
});

app.post('/channels/:channelName/peers', requireAdmin, function(req, res, next) {
	var peers = req.body.peers;
	if (!requireField(res, req.params.channelName, 'channelName') ||
		!requireField(res, peers, 'peers')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return join.joinChannel(
			req.params.channelName,
			peers,
			req.username,
			req.orgname
		);
	});
});

app.post('/chaincodes', requireAdmin, function(req, res, next) {
	var peers = req.body.peers;
	var chaincodeName = req.body.chaincodeName;
	var chaincodePath = req.body.chaincodePath;
	var chaincodeVersion = req.body.chaincodeVersion;
	if (!requireField(res, peers, 'peers') ||
		!requireField(res, chaincodeName, 'chaincodeName') ||
		!requireField(res, chaincodePath, 'chaincodePath') ||
		!requireField(res, chaincodeVersion, 'chaincodeVersion')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return install.installChaincode(
			peers,
			chaincodeName,
			chaincodePath,
			chaincodeVersion,
			req.username,
			req.orgname
		);
	});
});

app.post('/channels/:channelName/chaincodes', requireAdmin, function(req, res, next) {
	var peers = req.body.peers;
	var chaincodeName = req.body.chaincodeName;
	var chaincodeVersion = req.body.chaincodeVersion;
	var fcn = req.body.fcn || 'Init';
	var args = req.body.args;
	if (!requireField(res, peers, 'peers') ||
		!requireField(res, chaincodeName, 'chaincodeName') ||
		!requireField(res, chaincodeVersion, 'chaincodeVersion') ||
		!requireField(res, args, 'args')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return instantiate.instantiateChaincode(
			peers,
			req.params.channelName,
			chaincodeName,
			chaincodeVersion,
			fcn,
			args,
			req.username,
			req.orgname
		);
	});
});

app.post('/channels/:channelName/chaincodes/:chaincodeName', function(req, res, next) {
	var peers = req.body.peers;
	var fcn = req.body.fcn;
	var args = req.body.args;
	if (!requireField(res, peers, 'peers') ||
		!requireField(res, fcn, 'fcn') ||
		!requireField(res, args, 'args')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return invoke.invokeChaincode(
			peers,
			req.params.channelName,
			req.params.chaincodeName,
			fcn,
			args,
			req.username,
			req.orgname
		);
	});
});

app.get('/channels/:channelName/chaincodes/:chaincodeName', function(req, res, next) {
	var peer = req.query.peer;
	var fcn = req.query.fcn;
	if (!requireField(res, peer, 'peer') || !requireField(res, fcn, 'fcn') ||
		!requireField(res, req.query.args, 'args')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.queryChaincode(
			peer,
			req.params.channelName,
			req.params.chaincodeName,
			parseArgs(req.query.args),
			fcn,
			req.username,
			req.orgname
		);
	});
});

app.get('/channels/:channelName/blocks/:blockNumber', function(req, res, next) {
	if (!requireField(res, req.query.peer, 'peer')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.getBlockByNumber(
			req.query.peer,
			req.params.channelName,
			req.params.blockNumber,
			req.username,
			req.orgname
		);
	});
});

app.get('/channels/:channelName/blocks', function(req, res, next) {
	if (!requireField(res, req.query.peer, 'peer') ||
		!requireField(res, req.query.hash, 'hash')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.getBlockByHash(
			req.query.peer,
			req.params.channelName,
			req.query.hash,
			req.username,
			req.orgname
		);
	});
});

app.get('/channels/:channelName/transactions/:transactionId', function(req, res, next) {
	if (!requireField(res, req.query.peer, 'peer')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.getTransactionByID(
			req.query.peer,
			req.params.channelName,
			req.params.transactionId,
			req.username,
			req.orgname
		);
	});
});

app.get('/channels/:channelName', function(req, res, next) {
	if (!requireField(res, req.query.peer, 'peer')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.getChainInfo(
			req.query.peer,
			req.params.channelName,
			req.username,
			req.orgname
		);
	});
});

app.get('/chaincodes', function(req, res, next) {
	var type = req.query.type;
	if (!requireField(res, req.query.peer, 'peer') ||
		(type !== 'installed' && type !== 'instantiated')) {
		if (type !== 'installed' && type !== 'instantiated' && !res.headersSent) {
			res.status(400).json(errorResponse('\'type\''));
		}
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.getInstalledChaincodes(
			req.query.peer,
			req.query.channelName || hfc.getConfigSetting('channelName'),
			type,
			req.username,
			req.orgname
		);
	});
});

app.get('/channels', function(req, res, next) {
	if (!requireField(res, req.query.peer, 'peer')) {
		return;
	}
	sendFabricResult(res, next, req.orgname, function() {
		return query.getChannels(
			req.query.peer,
			req.username,
			req.orgname
		);
	});
});

app.use(function(err, req, res, next) {
	logger.error(err.stack || err);
	if (res.headersSent) {
		return next(err);
	}
	res.status(500).json({
		success: false,
		message: err.message || 'Internal server error'
	});
});

var server = http.createServer(app).listen(port, host, function() {
	logger.info('Yakusoku Ledger API listening at http://' + host + ':' + port);
});
server.timeout = 240000;

module.exports = app;
