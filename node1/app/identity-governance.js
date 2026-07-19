'use strict';

var crypto = require('crypto');
var fs = require('fs-extra');
var path = require('path');

var storePath = process.env.IDENTITY_GOVERNANCE_STORE ||
	path.join(__dirname, '../tmp/identity-governance.json');
var allowedRoles = {
	org1: ['university_reviewer', 'organization_admin'],
	org2: ['student', 'organization_admin']
};

function emptyStore() {
	return {version: 1, invitations: [], members: [], events: []};
}

function readStore() {
	if (!fs.existsSync(storePath)) {
		return emptyStore();
	}
	return JSON.parse(fs.readFileSync(storePath, 'utf8'));
}

function writeStore(store) {
	var directory = path.dirname(storePath);
	fs.ensureDirSync(directory);
	var temporary = storePath + '.tmp';
	fs.writeFileSync(temporary, JSON.stringify(store, null, 2), {mode: 384});
	fs.renameSync(temporary, storePath);
}

function now() {
	return new Date().toISOString();
}

function randomToken() {
	return crypto.randomBytes(32).toString('base64')
		.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function tokenHash(token) {
	return crypto.createHash('sha256').update(String(token)).digest('hex');
}

function assertRole(orgName, role) {
	if (!allowedRoles[orgName] || allowedRoles[orgName].indexOf(role) === -1) {
		throw new Error('Role is not valid for ' + orgName);
	}
}

function audit(store, type, details) {
	var event = Object.assign({type: type, timestamp: now()}, details || {});
	store.events.push(event);
	return event;
}

function refreshExpiry(invitation) {
	if ((invitation.status === 'issued' || invitation.status === 'claimed') &&
		new Date(invitation.expiresAt).getTime() <= Date.now()) {
		invitation.status = 'expired';
	}
}

function createInvitation(orgName, role, expiresInMinutes, actor) {
	assertRole(orgName, role);
	var duration = Number(expiresInMinutes);
	if (!Number.isInteger(duration) || duration < 5 || duration > 10080) {
		throw new Error('expiresInMinutes must be an integer from 5 to 10080');
	}
	var store = readStore();
	var token = randomToken();
	var invitation = {
		id: 'INV-' + crypto.randomBytes(8).toString('hex').toUpperCase(),
		orgName: orgName,
		role: role,
		tokenHash: tokenHash(token),
		status: 'issued',
		createdBy: actor,
		createdAt: now(),
		expiresAt: new Date(Date.now() + duration * 60000).toISOString()
	};
	store.invitations.push(invitation);
	audit(store, 'invitation.created', {
		orgName: orgName,
		invitationId: invitation.id,
		actor: actor,
		role: role
	});
	writeStore(store);
	return {invitation: publicInvitation(invitation), token: token};
}

function claimInvitation(token, orgName, username) {
	var store = readStore();
	var hash = tokenHash(token);
	var invitation = store.invitations.find(function(item) {
		refreshExpiry(item);
		return item.tokenHash === hash;
	});
	if (!invitation || invitation.orgName !== orgName || invitation.status !== 'issued') {
		writeStore(store);
		throw new Error('Invitation is invalid, expired, revoked, or already used');
	}
	invitation.status = 'claimed';
	invitation.claimedBy = username;
	invitation.claimedAt = now();
	audit(store, 'invitation.claimed', {
		orgName: orgName,
		invitationId: invitation.id,
		subject: username
	});
	writeStore(store);
	return publicInvitation(invitation);
}

function completeEnrollment(invitationId, username) {
	var store = readStore();
	var invitation = findInvitation(store, invitationId);
	if (invitation.status !== 'claimed' || invitation.claimedBy !== username) {
		throw new Error('Invitation claim is no longer valid');
	}
	invitation.status = 'used';
	invitation.usedBy = username;
	invitation.usedAt = now();
	var existing = store.members.find(function(member) {
		return member.orgName === invitation.orgName && member.username === username;
	});
	var member = existing || {};
	member.orgName = invitation.orgName;
	member.username = username;
	member.role = invitation.role;
	member.status = 'active';
	member.invitationId = invitation.id;
	member.enrolledAt = invitation.usedAt;
	if (!existing) {
		store.members.push(member);
	}
	audit(store, 'member.enrolled', {
		orgName: invitation.orgName,
		invitationId: invitation.id,
		subject: username,
		role: invitation.role
	});
	writeStore(store);
	return member;
}

function releaseClaim(invitationId, username, reason) {
	var store = readStore();
	var invitation = findInvitation(store, invitationId);
	if (invitation.status === 'claimed' && invitation.claimedBy === username) {
		invitation.status = 'issued';
		delete invitation.claimedBy;
		delete invitation.claimedAt;
		audit(store, 'invitation.claim_released', {
			orgName: invitation.orgName,
			invitationId: invitation.id,
			subject: username,
			reason: String(reason || 'enrollment failed')
		});
		writeStore(store);
	}
}

function listInvitations(orgName) {
	var store = readStore();
	store.invitations.forEach(refreshExpiry);
	writeStore(store);
	return store.invitations.filter(function(item) {
		return item.orgName === orgName;
	}).map(publicInvitation);
}

function revokeInvitation(orgName, invitationId, actor, reason) {
	var store = readStore();
	var invitation = findInvitation(store, invitationId);
	if (invitation.orgName !== orgName || invitation.status === 'used') {
		throw new Error('Only an unused invitation in your organization may be revoked');
	}
	invitation.status = 'revoked';
	invitation.revokedBy = actor;
	invitation.revokedAt = now();
	invitation.revocationReason = String(reason || 'revoked by administrator');
	audit(store, 'invitation.revoked', {
		orgName: orgName,
		invitationId: invitation.id,
		actor: actor,
		reason: invitation.revocationReason
	});
	writeStore(store);
	return publicInvitation(invitation);
}

function reissueInvitation(orgName, invitationId, expiresInMinutes, actor) {
	var store = readStore();
	var invitation = findInvitation(store, invitationId);
	if (invitation.orgName !== orgName) {
		throw new Error('Invitation does not belong to your organization');
	}
	if (invitation.status === 'issued' || invitation.status === 'claimed') {
		invitation.status = 'revoked';
		invitation.revokedBy = actor;
		invitation.revokedAt = now();
		invitation.revocationReason = 'reissued';
		writeStore(store);
	}
	return createInvitation(orgName, invitation.role, expiresInMinutes, actor);
}

function listMembers(orgName) {
	return readStore().members.filter(function(member) {
		return member.orgName === orgName;
	});
}

function getMember(orgName, username) {
	return readStore().members.find(function(member) {
		return member.orgName === orgName && member.username === username;
	}) || null;
}

function revokeMember(orgName, username, actor, reason) {
	var store = readStore();
	var member = store.members.find(function(item) {
		return item.orgName === orgName && item.username === username;
	});
	if (!member || member.status !== 'active') {
		throw new Error('Active member does not exist');
	}
	member.status = 'revoked';
	member.revokedBy = actor;
	member.revokedAt = now();
	member.revocationReason = String(reason || 'revoked by administrator');
	audit(store, 'member.revoked', {
		orgName: orgName,
		actor: actor,
		subject: username,
		reason: member.revocationReason
	});
	writeStore(store);
	return member;
}

function listEvents(orgName) {
	return readStore().events.filter(function(event) {
		return event.orgName === orgName;
	});
}

function publicInvitation(invitation) {
	var copy = Object.assign({}, invitation);
	delete copy.tokenHash;
	return copy;
}

function findInvitation(store, invitationId) {
	var invitation = store.invitations.find(function(item) {
		return item.id === invitationId;
	});
	if (!invitation) {
		throw new Error('Invitation does not exist');
	}
	return invitation;
}

exports.allowedRoles = allowedRoles;
exports.createInvitation = createInvitation;
exports.claimInvitation = claimInvitation;
exports.completeEnrollment = completeEnrollment;
exports.releaseClaim = releaseClaim;
exports.listInvitations = listInvitations;
exports.revokeInvitation = revokeInvitation;
exports.reissueInvitation = reissueInvitation;
exports.listMembers = listMembers;
exports.getMember = getMember;
exports.revokeMember = revokeMember;
exports.listEvents = listEvents;
