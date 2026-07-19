'use strict';

const previewAgreements = [
  { id: 'AGR-2026-0248ABCD0001', student: 'Aiko Tanaka', email: 'aiko.tanaka@example.edu', university: 'Kyoto International University', amountMinor: 680000, currency: 'JPY', status: 'Approved', date: '2026-07-16', preview: true, verified: true },
  { id: 'AGR-2026-0247BCDE0002', student: 'Haruto Sato', email: 'haruto.sato@example.edu', university: 'University of Tokyo', amountMinor: 920000, currency: 'JPY', status: 'Submitted', date: '2026-07-15', preview: true, verified: true },
  { id: 'AGR-2026-0246CDEF0003', student: 'Mei Nakamura', email: 'mei.n@example.edu', university: 'Waseda University', amountMinor: 540000, currency: 'JPY', status: 'Submitted', date: '2026-07-14', preview: true, verified: false },
  { id: 'AGR-2026-0245DEFA0004', student: 'Ren Kobayashi', email: 'ren.k@example.edu', university: 'Osaka Metropolitan University', amountMinor: 760000, currency: 'JPY', status: 'Rejected', date: '2026-07-12', preview: true, verified: true }
];

const state = {
  agreements: [...previewAgreements],
  token: localStorage.getItem('yakusoku.token') || '',
  identity: readStoredIdentity(),
  documentHash: '',
  verifyHash: ''
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];

document.addEventListener('DOMContentLoaded', () => {
  $('#today').textContent = new Intl.DateTimeFormat('en', { month: 'long', day: 'numeric', year: 'numeric' }).format(new Date()).toUpperCase();
  $('#createForm').elements.date.valueAsDate = new Date();
  bindNavigation();
  bindAuthentication();
  bindAgreements();
  bindDocuments();
  renderAgreements();
  updateIdentityUI();
});

function readStoredIdentity() {
  try {
    return JSON.parse(localStorage.getItem('yakusoku.identity') || 'null');
  } catch {
    return null;
  }
}

function bindNavigation() {
  const closeMenu = () => {
    document.body.classList.remove('menu-open');
    $('#menuButton').setAttribute('aria-expanded', 'false');
  };

  $('#menuButton').addEventListener('click', () => {
    const open = document.body.classList.toggle('menu-open');
    $('#menuButton').setAttribute('aria-expanded', String(open));
  });
  $('#mobileOverlay').addEventListener('click', closeMenu);
  $$('.nav-link').forEach(link => link.addEventListener('click', closeMenu));
  $$('[data-scroll]').forEach(button => button.addEventListener('click', () => {
    document.getElementById(button.dataset.scroll).scrollIntoView({ behavior: 'smooth' });
  }));

  const sections = $$('.section-anchor');
  const observer = new IntersectionObserver(entries => {
    const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
    if (!visible) return;
    $$('.nav-link').forEach(link => link.classList.toggle('active', link.dataset.section === visible.target.id));
    const active = $(`.nav-link[data-section="${visible.target.id}"]`);
    if (active) $('#pageTitle').textContent = active.textContent.trim();
  }, { rootMargin: '-15% 0px -70%', threshold: [0, .2, .5] });
  sections.forEach(section => observer.observe(section));
}

function bindAuthentication() {
  const dialog = $('#authDialog');
  const open = () => {
    if (state.token) {
      signOut();
      return;
    }
    dialog.showModal();
    setTimeout(() => $('#authForm').elements.username.focus(), 0);
  };

  $('#authButton').addEventListener('click', open);
  $$('[data-open-auth]').forEach(button => button.addEventListener('click', () => {
    if (state.token) signOut();
    else dialog.showModal();
  }));
  $('#authForm').addEventListener('submit', async event => {
    event.preventDefault();
    const form = event.currentTarget;
    const status = $('#authStatus');
    const submit = form.querySelector('[type="submit"]');
    setStatus(status, 'Connecting to the Fabric certificate authority…');
    submit.disabled = true;

    const values = new FormData(form);
    const body = {
      username: String(values.get('username')).trim(),
      orgName: values.get('orgName')
    };
    const secret = String(values.get('adminSecret') || '');
    if (secret) body.adminSecret = secret;
    const organizationSecret = String(values.get('organizationSecret') || '');
    if (organizationSecret) body.organizationSecret = organizationSecret;

    try {
      const response = await apiRequest('/users', { method: 'POST', body, authenticated: false });
      if (!response.token) throw new Error('The enrollment response did not include an authentication token.');
      const claims = decodeToken(response.token);
      state.token = response.token;
      state.identity = {
        username: body.username,
        orgName: body.orgName,
        role: claims.role || 'user'
      };
      localStorage.setItem('yakusoku.token', state.token);
      localStorage.setItem('yakusoku.identity', JSON.stringify(state.identity));
      setStatus(status, 'Identity connected.', 'success');
      updateIdentityUI();
      addActivity('Identity connected', `${body.username} joined through ${body.orgName}.`, '✓', 'green');
      showToast(`Connected as ${body.username}`);
      await loadAgreements();
      setTimeout(() => dialog.close(), 450);
    } catch (error) {
      setStatus(status, error.message, 'error');
    } finally {
      submit.disabled = false;
    }
  });
}

function decodeToken(token) {
  try {
    const part = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(decodeURIComponent(atob(part).split('').map(char => `%${char.charCodeAt(0).toString(16).padStart(2, '0')}`).join('')));
  } catch {
    return {};
  }
}

function signOut() {
  state.token = '';
  state.identity = null;
  localStorage.removeItem('yakusoku.token');
  localStorage.removeItem('yakusoku.identity');
  state.agreements = [...previewAgreements];
  updateIdentityUI();
  renderAgreements();
  showToast('Signed out. Preview data is still available.');
}

function updateIdentityUI() {
  const connected = Boolean(state.token && state.identity);
  const copy = $('.auth-copy');
  const avatar = $('.avatar');
  if (connected) {
    copy.innerHTML = `<strong>${escapeHtml(state.identity.username)}</strong><small>${escapeHtml(state.identity.orgName)} · ${escapeHtml(state.identity.role)}</small>`;
    avatar.textContent = state.identity.username.slice(0, 1).toUpperCase();
    $('#authButton').title = 'Sign out';
    $('.preview-banner p').textContent = 'Preview records remain clearly labeled. Authenticated transactions will be marked as live ledger records.';
    $('.preview-banner button').textContent = 'Sign out';
  } else {
    copy.innerHTML = '<strong>Sign in</strong><small>Connect identity</small>';
    avatar.textContent = '入';
    $('#authButton').title = 'Connect identity';
    $('.preview-banner p').textContent = 'Sample records are shown until you authenticate. Ledger mutations always require a verified identity.';
    $('.preview-banner button').textContent = 'Connect now';
  }
  renderReviewQueue();
}

function bindAgreements() {
  $('#agreementSearch').addEventListener('input', renderAgreements);
  $('#refreshButton').addEventListener('click', async () => {
    if (!requireAuthentication('refresh ledger records')) return;
    await loadAgreements();
  });

  $('#createForm').addEventListener('submit', async event => {
    event.preventDefault();
    const form = event.currentTarget;
    const status = $('#createStatus');
    if (!requireAuthentication('submit an agreement')) return setStatus(status, 'Sign in before submitting a ledger transaction.', 'error');
    if (!state.documentHash) return setStatus(status, 'Choose a document and wait for its SHA-256 fingerprint.', 'error');

    const values = Object.fromEntries(new FormData(form));
    const submit = form.querySelector('[type="submit"]');
    submit.disabled = true;
    setStatus(status, 'Submitting transaction proposal to Fabric…');
    try {
      const result = await apiRequest('/api/agreements', {
        method: 'POST',
        body: {
          studentName: values.studentName.trim(),
          email: values.email.trim(),
          date: values.date,
          amount: String(values.amount),
          currency: values.currency,
          universityName: values.university.trim(),
          documentHash: state.documentHash
        }
      });
      await loadAgreements();
      form.reset();
      form.elements.date.valueAsDate = new Date();
      state.documentHash = '';
      $('#createHash').textContent = 'SHA-256 fingerprint will appear here.';
      $('#createHash').classList.remove('ready');
      setStatus(status, transactionMessage(result), 'success');
      addActivity('Agreement submitted to ledger', `${values.studentName} · ${values.university}`, '文', 'vermilion');
      showToast('Agreement transaction committed.');
    } catch (error) {
      setStatus(status, error.message, 'error');
    } finally {
      submit.disabled = false;
    }
  });
}

function renderAgreements() {
  const term = $('#agreementSearch').value.trim().toLowerCase();
  const records = state.agreements.filter(item => [item.id, item.student, item.email, item.university, item.status].some(value => value.toLowerCase().includes(term)));
  $('#agreementRows').innerHTML = records.map(item => `
    <tr>
      <td data-label="Agreement"><span class="agreement-id"><i class="agreement-glyph">約</i>${escapeHtml(item.id)}${item.preview ? '<em class="preview-tag">PREVIEW</em>' : ''}</span></td>
      <td data-label="Student"><span class="student-cell"><strong>${escapeHtml(item.student)}</strong><small>${escapeHtml(item.email)}</small></span></td>
      <td data-label="University"><span class="university">${escapeHtml(item.university)}</span></td>
      <td data-label="Value"><span class="amount">${formatMoney(item.amountMinor, item.currency)}</span></td>
      <td data-label="Status"><span class="status ${item.status.toLowerCase()}">${escapeHtml(item.status)}</span></td>
      <td data-label="Date">${formatDate(item.date)}</td>
      <td><button class="row-menu" type="button" aria-label="Details for ${escapeHtml(item.id)}" data-agreement="${escapeHtml(item.id)}">•••</button></td>
    </tr>`).join('');
  $('#agreementEmpty').hidden = records.length > 0;
  $('#recordCount').textContent = `Showing ${records.length} ${records.every(record => record.preview) ? 'preview ' : ''}record${records.length === 1 ? '' : 's'}`;
  $$('[data-agreement]').forEach(button => button.addEventListener('click', () => showAgreementHistory(button.dataset.agreement)));
  renderStats();
  renderReviewQueue();
  renderVerifyOptions();
}

function renderStats() {
  if (!state.identity) {
    $('[data-stat="total"]').textContent = '248';
    $('[data-stat="pending"]').textContent = '17';
    $('[data-stat="value"]').textContent = '¥1.84M';
    $('[data-stat="verified"]').textContent = '231';
    return;
  }
  $('[data-stat="total"]').textContent = String(state.agreements.length);
  $('[data-stat="pending"]').textContent = String(state.agreements.filter(item => item.status === 'Submitted').length);
  $('[data-stat="value"]').textContent = compactApprovedValue(state.agreements);
  $('[data-stat="verified"]').textContent = String(state.agreements.filter(item => item.verified).length);
}

function renderReviewQueue() {
  const submitted = state.agreements.filter(item => item.status === 'Submitted');
  $('#reviewCount').textContent = String(submitted.length);
  const universityReviewer = state.identity?.orgName === 'org1';
  $('#workflowMessage').textContent = universityReviewer
    ? 'University review controls are enabled. Decisions are committed to the ledger.'
    : 'Connect a University organization identity to approve or reject submitted agreements.';
  $('#reviewQueue').innerHTML = submitted.map(item => `
    <article class="review-item">
      <header><strong>${escapeHtml(item.id)}</strong><small>${escapeHtml(item.student)}</small></header>
      ${universityReviewer && !item.preview ? `<div class="review-actions"><button type="button" data-decision="Approved" data-id="${escapeHtml(item.id)}">Approve</button><button type="button" data-decision="Rejected" data-id="${escapeHtml(item.id)}">Reject</button></div>` : ''}
    </article>`).join('');
  $$('[data-decision]').forEach(button => button.addEventListener('click', () => decideAgreement(button.dataset.id, button.dataset.decision, button)));
}

async function decideAgreement(id, decision, button) {
  if (!requireAuthentication('review agreements') || state.identity?.orgName !== 'org1') return;
  button.disabled = true;
  try {
    await apiRequest(`/api/agreements/${encodeURIComponent(id)}/review`, {
      method: 'POST',
      body: { decision: decision.toLowerCase() }
    });
    await loadAgreements();
    addActivity(`Agreement ${decision.toLowerCase()}`, `${id} was updated by ${state.identity.username}.`, decision === 'Approved' ? '✓' : '×', decision === 'Approved' ? 'green' : 'vermilion');
    showToast(`${id} marked ${decision.toLowerCase()}.`);
  } catch (error) {
    showToast(error.message, true);
    button.disabled = false;
  }
}

function bindDocuments() {
  $('#agreementDocument').addEventListener('change', async event => {
    const file = event.target.files[0];
    if (!file) return;
    $('.file-drop b').textContent = file.name;
    $('#createHash').textContent = 'Calculating SHA-256…';
    try {
      state.documentHash = await sha256(file);
      $('#createHash').textContent = `SHA-256 · ${state.documentHash}`;
      $('#createHash').classList.add('ready');
    } catch (error) {
      setStatus($('#createStatus'), error.message, 'error');
    }
  });

  $('#verifyFile').addEventListener('change', async event => {
    const file = event.target.files[0];
    if (!file) return;
    $('.verify-drop b').textContent = file.name;
    $('#verifyHash').textContent = 'Calculating SHA-256…';
    $('#verifyButton').disabled = true;
    setStatus($('#verifyStatus'), '');
    try {
      state.verifyHash = await sha256(file);
      $('#verifyHash').textContent = `SHA-256 · ${state.verifyHash}`;
      $('#verifyHash').classList.add('ready');
      $('#verifyButton').disabled = false;
    } catch (error) {
      setStatus($('#verifyStatus'), error.message, 'error');
    }
  });

  $('#verifyButton').addEventListener('click', async event => {
    const status = $('#verifyStatus');
    if (!requireAuthentication('compare a document with the ledger')) return setStatus(status, 'Sign in before querying the ledger.', 'error');
    event.currentTarget.disabled = true;
    setStatus(status, 'Querying the ledger fingerprint…');
    try {
      const agreementId = $('#verifyAgreement').value;
      if (!agreementId) throw new Error('Select the agreement whose document you want to verify.');
      const result = await apiRequest(`/api/agreements/${encodeURIComponent(agreementId)}/verify`, {
        method: 'POST',
        body: { documentHash: state.verifyHash }
      });
      const verified = interpretVerification(result);
      setStatus(status, verified ? 'Verified — this fingerprint exists on the ledger.' : 'No matching fingerprint was found.', verified ? 'success' : 'error');
      addActivity(verified ? 'Document verified' : 'Document not found', `${state.verifyHash.slice(0, 16)}… checked against the ledger.`, verified ? '✓' : '×', verified ? 'green' : 'vermilion');
    } catch (error) {
      setStatus(status, error.message, 'error');
    } finally {
      event.currentTarget.disabled = false;
    }
  });

  $('#clearActivity').addEventListener('click', () => {
    $('.notification-button i').hidden = true;
    showToast('Notifications marked as read.');
  });
}

async function sha256(file) {
  if (!window.crypto?.subtle) throw new Error('Secure browser hashing is unavailable. Open this page over HTTPS or localhost.');
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  return [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, '0')).join('');
}

async function loadAgreements() {
  try {
    const records = await apiRequest('/api/agreements');
    state.agreements = Array.isArray(records) ? records.map(normalizeAgreement) : [];
    renderAgreements();
    showToast(`Loaded ${state.agreements.length} live ledger record${state.agreements.length === 1 ? '' : 's'}.`);
  } catch (error) {
    showToast(error.message, true);
  }
}

function normalizeAgreement(record) {
  const value = record.Value || record;
  const status = String(value.Status || 'submitted');
  return {
    id: record.Key || value.Key,
    ledgerId: record.Key || value.Key,
    student: String(value.StudentName || 'Private data unavailable'),
    email: String(value.Email || (value.PrivacyVersion ? 'Private data unavailable' : '')),
    university: value.UniversityName,
    amountMinor: value.AmountMinor !== undefined
      ? Number(value.AmountMinor)
      : legacyAmountMinor(value.Amount, value.Currency || 'JPY'),
    currency: String(value.Currency || 'JPY'),
    status: status.charAt(0).toUpperCase() + status.slice(1),
    date: value.Date,
    preview: false,
    verified: Boolean(value.DocumentHash),
    documentHash: value.DocumentHash || ''
  };
}

function renderVerifyOptions() {
  const current = $('#verifyAgreement').value;
  const live = state.agreements.filter(item => !item.preview && item.documentHash);
  $('#verifyAgreement').innerHTML = '<option value="">Select a live agreement</option>' + live.map(item =>
    `<option value="${escapeHtml(item.id)}">${escapeHtml(item.student)} · ${escapeHtml(item.university)}</option>`
  ).join('');
  if (live.some(item => item.id === current)) $('#verifyAgreement').value = current;
}

async function showAgreementHistory(id) {
  const agreement = state.agreements.find(item => item.id === id);
  $('#historySubtitle').textContent = agreement ? `${agreement.student} · ${agreement.university}` : id;
  const list = $('#historyList');
  if (agreement?.preview) {
    list.innerHTML = `
      <li><strong>${escapeHtml(agreement.status)}</strong><span>Preview agreement state</span><small>${escapeHtml(agreement.date)}</small></li>
      <li><strong>Submitted</strong><span>Agreement proposal created</span><small>Preview transaction</small></li>`;
    $('#historyDialog').showModal();
    return;
  }
  if (!requireAuthentication('view agreement history')) return;
  list.innerHTML = '<li><span>Loading ledger history…</span></li>';
  $('#historyDialog').showModal();
  try {
    const history = await apiRequest(`/api/agreements/${encodeURIComponent(id)}/history`);
    list.innerHTML = history.length ? history.slice().reverse().map(entry => {
      const value = entry.Value || {};
      const status = entry.IsDelete ? 'Deleted' : String(value.Status || 'Submitted');
      const amountMinor = value.AmountMinor !== undefined
        ? Number(value.AmountMinor)
        : legacyAmountMinor(value.Amount, value.Currency || 'JPY');
      return `<li><strong>${escapeHtml(status)}</strong><span>${entry.IsDelete ? 'Agreement removed' : `Value ${formatMoney(amountMinor, value.Currency || 'JPY')}`}</span><small>${escapeHtml(entry.Timestamp)} · ${escapeHtml(entry.TxId)}</small></li>`;
    }).join('') : '<li><span>No history entries were returned.</span></li>';
  } catch (error) {
    list.innerHTML = `<li><span>${escapeHtml(error.message)}</span></li>`;
  }
}

async function apiRequest(path, options = {}) {
  const authenticated = options.authenticated !== false;
  if (authenticated && !state.token) throw new Error('Authentication is required for this ledger request.');
  const headers = { Accept: 'application/json' };
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  if (authenticated) headers.Authorization = `Bearer ${state.token}`;

  let response;
  try {
    response = await fetch(path, {
      method: options.method || 'GET',
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    });
  } catch {
    throw new Error('The ledger API could not be reached. Check that the same-origin server is running.');
  }

  const contentType = response.headers.get('content-type') || '';
  const payload = contentType.includes('json') ? await response.json().catch(() => ({})) : await response.text();
  if (!response.ok) {
    if (response.status === 401 && authenticated) signOut();
    const message = typeof payload === 'string' ? payload : payload.message || payload.error;
    throw new Error(message || `Ledger request failed (${response.status}).`);
  }
  if (payload && typeof payload === 'object' && payload.success === false) {
    throw new Error(payload.message || 'The ledger rejected this request.');
  }
  return payload;
}

function interpretVerification(result) {
  if (typeof result === 'boolean') return result;
  if (typeof result === 'string') {
    const normalized = result.trim().toLowerCase();
    if (['true', 'verified', 'match', '1'].includes(normalized)) return true;
    try { return interpretVerification(JSON.parse(result)); } catch { return false; }
  }
  return Boolean(result?.verified ?? result?.match ?? result?.exists);
}

function requireAuthentication(action) {
  if (state.token) return true;
  showToast(`Sign in to ${action}.`, true);
  $('#authDialog').showModal();
  return false;
}

function setStatus(element, message, type = '') {
  element.textContent = message;
  element.className = `form-status ${type}`.trim();
}

function addActivity(title, detail, glyph, color) {
  const item = document.createElement('li');
  item.innerHTML = `<span class="timeline-icon ${color}">${glyph}</span><div><strong>${escapeHtml(title)}</strong><p>${escapeHtml(detail)}</p><time datetime="${new Date().toISOString()}">Just now</time></div>`;
  $('#activityList').prepend(item);
  $('.notification-button i').hidden = false;
}

function showToast(message, error = false) {
  const toast = document.createElement('div');
  toast.className = `toast${error ? ' error' : ''}`;
  toast.setAttribute('role', error ? 'alert' : 'status');
  toast.textContent = message;
  $('#toastRegion').append(toast);
  setTimeout(() => toast.remove(), 4500);
}

function transactionMessage(result) {
  if (typeof result === 'string') return result;
  return result?.message || 'Transaction committed successfully.';
}

function currencyScale(currency) {
  return ['JPY', 'KRW'].includes(currency) ? 0 : 2;
}

function legacyAmountMinor(amount, currency) {
  const numeric = Number(amount || 0);
  return Math.round(numeric * (10 ** currencyScale(currency)));
}

function formatMoney(amountMinor, currency) {
  const scale = currencyScale(currency);
  const major = Number(amountMinor || 0) / (10 ** scale);
  return new Intl.NumberFormat('en', {
    style: 'currency',
    currency,
    minimumFractionDigits: scale,
    maximumFractionDigits: scale
  }).format(major);
}

function compactApprovedValue(agreements) {
  const totals = agreements
    .filter(item => item.status === 'Approved')
    .reduce((result, item) => {
      result[item.currency] = (result[item.currency] || 0) + item.amountMinor;
      return result;
    }, {});
  const currencies = Object.keys(totals);
  if (!currencies.length) return '—';
  if (currencies.length > 1) return `${currencies.length} currencies`;
  const currency = currencies[0];
  const major = totals[currency] / (10 ** currencyScale(currency));
  return new Intl.NumberFormat('en', {
    style: 'currency',
    currency,
    notation: 'compact',
    maximumFractionDigits: 2
  }).format(major);
}

function formatDate(value) {
  const date = new Date(`${value}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return 'Invalid date';
  return new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }).format(date);
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
}
