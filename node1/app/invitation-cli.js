'use strict';

var governance = require('./identity-governance.js');

var command = process.argv[2];
var orgName = process.argv[3];
var role = process.argv[4];
var duration = process.argv[5] || '60';

if (command !== 'create' || !orgName || !role) {
	console.error('Usage: node app/invitation-cli.js create <org1|org2> <role> [minutes]');
	process.exit(1);
}

try {
	var result = governance.createInvitation(orgName, role, Number(duration), 'local-bootstrap');
	console.log(JSON.stringify(result));
} catch (err) {
	console.error(err.message);
	process.exit(1);
}
