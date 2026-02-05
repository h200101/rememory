// ReMemory Recovery Tool - Browser-based recovery using Go WASM

(function() {
  'use strict';

  // Import shared utilities
  const { escapeHtml, formatSize, toast, showInlineError, clearInlineError } = window.rememoryUtils;

  // State
  const state = {
    shares: [],        // Array of parsed share objects
    manifest: null,    // Uint8Array of MANIFEST.age content
    threshold: 0,      // Required shares (from first parsed share)
    total: 0,          // Total shares
    wasmReady: false,
    recovering: false,
    recoveryComplete: false
  };

  // DOM elements
  const elements = {
    loadingOverlay: document.getElementById('loading-overlay'),
    shareDropZone: document.getElementById('share-drop-zone'),
    shareFileInput: document.getElementById('share-file-input'),
    sharesList: document.getElementById('shares-list'),
    thresholdInfo: document.getElementById('threshold-info'),
    manifestDropZone: document.getElementById('manifest-drop-zone'),
    manifestFileInput: document.getElementById('manifest-file-input'),
    manifestStatus: document.getElementById('manifest-status'),
    recoverBtn: document.getElementById('recover-btn'),
    recoverSection: document.getElementById('recover-section'),
    progressBar: document.getElementById('progress-bar'),
    statusMessage: document.getElementById('status-message'),
    filesList: document.getElementById('files-list'),
    downloadActions: document.getElementById('download-actions'),
    downloadAllBtn: document.getElementById('download-all-btn'),
    pasteToggleBtn: document.getElementById('paste-toggle-btn'),
    pasteArea: document.getElementById('paste-area'),
    pasteInput: document.getElementById('paste-input'),
    pasteSubmitBtn: document.getElementById('paste-submit-btn'),
    contactListSection: document.getElementById('contact-list-section'),
    contactList: document.getElementById('contact-list'),
    step1Card: null, // Set after DOM ready
    step2Card: null  // Set after DOM ready
  };

  // Personalization data (embedded in HTML)
  const personalization = (typeof PERSONALIZATION !== 'undefined') ? PERSONALIZATION : null;

  // Share regex to extract from README.txt content
  const shareRegex = /-----BEGIN REMEMORY SHARE-----([\s\S]*?)-----END REMEMORY SHARE-----/;

  // Initialize
  async function init() {
    // Get step card references
    const cards = document.querySelectorAll('.card');
    elements.step1Card = cards[0];
    elements.step2Card = cards[1];

    setupDropZones();
    setupButtons();
    setupPaste();

    // Render contact list immediately (doesn't need WASM)
    if (personalization && personalization.otherFriends && personalization.otherFriends.length > 0) {
      renderContactList();
      elements.contactListSection.classList.remove('hidden');
    }

    await loadWasm();

    // Load personalization data after WASM is ready
    if (personalization) {
      loadPersonalizationData();
    }
  }

  // Load personalization data (holder's share only - manifest must be loaded separately)
  function loadPersonalizationData() {
    if (!personalization) return;

    // Load the holder's share automatically
    if (personalization.holderShare) {
      const result = window.rememoryParseShare(personalization.holderShare);
      if (!result.error) {
        const share = result.share;
        share.isHolder = true; // Mark as holder's own share
        state.threshold = share.threshold;
        state.total = share.total;
        state.shares.push(share);
        updateSharesUI();
        updateContactList();
      }
    }

    checkRecoverReady();
  }

  // Render the contact list for other friends
  function renderContactList() {
    if (!personalization || !personalization.otherFriends) return;

    elements.contactList.innerHTML = '';

    personalization.otherFriends.forEach(friend => {
      const item = document.createElement('div');
      item.className = 'contact-item';
      item.dataset.name = friend.name;

      let contactInfo = '';
      if (friend.email) {
        contactInfo += `<a href="mailto:${escapeHtml(friend.email)}">${escapeHtml(friend.email)}</a>`;
      }
      if (friend.phone) {
        if (contactInfo) contactInfo += ' &bull; ';
        contactInfo += escapeHtml(friend.phone);
      }

      item.innerHTML = `
        <div class="checkbox"></div>
        <div class="details">
          <div class="name">${escapeHtml(friend.name)}</div>
          <div class="contact-info">${contactInfo || '—'}</div>
        </div>
      `;

      elements.contactList.appendChild(item);
    });
  }

  // Update contact list checkboxes based on collected shares
  function updateContactList() {
    if (!personalization || !personalization.otherFriends) return;

    const collectedNames = new Set(state.shares.map(s => s.holder?.toLowerCase()));

    elements.contactList.querySelectorAll('.contact-item').forEach(item => {
      const name = item.dataset.name.toLowerCase();
      const isCollected = collectedNames.has(name);
      item.classList.toggle('collected', isCollected);
      item.querySelector('.checkbox').textContent = isCollected ? '✓' : '';
    });
  }

  // Load WASM module
  async function loadWasm() {
    try {
      const go = new Go();
      const result = await WebAssembly.instantiateStreaming(
        fetch('recover.wasm'),
        go.importObject
      );
      go.run(result.instance);

      // Wait for WASM to signal ready
      await waitForWasm();
      state.wasmReady = true;
      window.rememoryAppReady = true;
      elements.loadingOverlay.classList.add('hidden');
    } catch (err) {
      // Try loading from embedded gzip-compressed base64 as fallback
      if (typeof WASM_BINARY !== 'undefined') {
        try {
          const go = new Go();
          const bytes = await decodeAndDecompressWasm(WASM_BINARY);
          const result = await WebAssembly.instantiate(bytes, go.importObject);
          go.run(result.instance);
          await waitForWasm();
          state.wasmReady = true;
          window.rememoryAppReady = true;
          elements.loadingOverlay.classList.add('hidden');
          return;
        } catch (e) {
          // WASM initialization failed
          errorHandlers.wasmLoadFailed(e);
          return;
        }
      }
      errorHandlers.wasmLoadFailed(err);
    }
  }

  function waitForWasm() {
    return new Promise((resolve) => {
      const check = () => {
        if (window.rememoryReady) {
          resolve();
        } else {
          setTimeout(check, 50);
        }
      };
      check();
    });
  }

  // Decode base64 and decompress gzip-compressed WASM
  async function decodeAndDecompressWasm(base64) {
    // Decode base64 to get gzip-compressed data
    const compressed = Uint8Array.from(atob(base64), c => c.charCodeAt(0));

    // Decompress using DecompressionStream (modern browsers)
    if (typeof DecompressionStream !== 'undefined') {
      const ds = new DecompressionStream('gzip');
      const writer = ds.writable.getWriter();
      writer.write(compressed);
      writer.close();
      const reader = ds.readable.getReader();
      const chunks = [];
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        chunks.push(value);
      }
      const totalLength = chunks.reduce((acc, chunk) => acc + chunk.length, 0);
      const bytes = new Uint8Array(totalLength);
      let offset = 0;
      for (const chunk of chunks) {
        bytes.set(chunk, offset);
        offset += chunk.length;
      }
      return bytes.buffer;
    } else if (typeof pako !== 'undefined') {
      // Fallback: use pako if available
      return pako.inflate(compressed).buffer;
    } else {
      throw new Error('Browser does not support DecompressionStream');
    }
  }

  // Setup drop zones
  function setupDropZones() {
    // Share drop zone
    setupDropZone(elements.shareDropZone, elements.shareFileInput, handleShareFiles);

    // Manifest drop zone
    setupDropZone(elements.manifestDropZone, elements.manifestFileInput, handleManifestFiles);
  }

  function setupDropZone(dropZone, fileInput, handler) {
    dropZone.addEventListener('click', () => fileInput.click());

    dropZone.addEventListener('dragover', (e) => {
      e.preventDefault();
      dropZone.classList.add('dragover');
    });

    dropZone.addEventListener('dragleave', () => {
      dropZone.classList.remove('dragover');
    });

    dropZone.addEventListener('drop', (e) => {
      e.preventDefault();
      dropZone.classList.remove('dragover');
      handler(e.dataTransfer.files);
    });

    fileInput.addEventListener('change', async (e) => {
      const files = Array.from(e.target.files);
      e.target.value = ''; // Reset for re-selection
      await handler(files);
    });
  }

  // Setup paste functionality
  function setupPaste() {
    elements.pasteToggleBtn.addEventListener('click', () => {
      const isHidden = elements.pasteArea.classList.contains('hidden');
      elements.pasteArea.classList.toggle('hidden', !isHidden);
      if (isHidden) {
        elements.pasteInput.focus();
      }
    });

    elements.pasteSubmitBtn.addEventListener('click', async () => {
      const content = elements.pasteInput.value.trim();
      if (!content) return;

      await parseAndAddShareFromPaste(content);
      elements.pasteInput.value = '';
      elements.pasteArea.classList.add('hidden');
    });

    // Allow Enter key in textarea with Ctrl/Cmd to submit
    elements.pasteInput.addEventListener('keydown', (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        elements.pasteSubmitBtn.click();
      }
    });
  }

  // Parse share from pasted content
  async function parseAndAddShareFromPaste(content) {
    if (!state.wasmReady) {
      toast.warning(t('error_not_ready_title'), t('error_not_ready_message'), t('error_not_ready_guidance'));
      return;
    }

    // Clear any previous inline errors
    clearInlineError(elements.shareDropZone);

    // Check if content contains a share
    if (!shareRegex.test(content)) {
      showError(
        t('error_paste_no_share_message'),
        {
          title: t('error_paste_no_share_title'),
          guidance: t('error_paste_no_share_guidance')
        }
      );
      return;
    }

    const result = window.rememoryParseShare(content);
    if (result.error) {
      showError(
        t('error_invalid_share_message', t('pasted_content')),
        {
          title: t('error_invalid_share_title'),
          guidance: t('error_invalid_share_guidance')
        }
      );
      return;
    }

    const share = result.share;

    // Check for duplicate index
    if (state.shares.some(s => s.index === share.index)) {
      errorHandlers.duplicateShare(share.index);
      return;
    }

    // Set threshold/total from first share
    if (state.shares.length === 0) {
      state.threshold = share.threshold;
      state.total = share.total;
    }

    state.shares.push(share);
    updateSharesUI();
    checkRecoverReady();
  }

  // Handle share file uploads
  async function handleShareFiles(files) {
    // Clear any previous inline errors
    clearInlineError(elements.shareDropZone);

    for (const file of files) {
      try {
        // Check if it's a ZIP file (bundle)
        if (file.name.endsWith('.zip') || file.type === 'application/zip') {
          await handleBundleZip(file);
        } else {
          const content = await readFileAsText(file);
          await parseAndAddShare(content, file.name);
        }
      } catch (err) {
        errorHandlers.fileReadFailed(file.name);
      }
    }
  }

  // Handle bundle ZIP file - extract share and optionally manifest
  async function handleBundleZip(file) {
    if (!state.wasmReady) {
      toast.warning(t('error_not_ready_title'), t('error_not_ready_message'), t('error_not_ready_guidance'));
      return;
    }

    const buffer = await readFileAsArrayBuffer(file);
    const zipData = new Uint8Array(buffer);

    const result = window.rememoryExtractBundle(zipData);
    if (result.error) {
      showError(
        t('error_bundle_extract_message', file.name),
        {
          title: t('error_bundle_extract_title'),
          guidance: t('error_bundle_extract_guidance'),
          inline: true,
          targetElement: elements.shareDropZone
        }
      );
      return;
    }

    // Add the share
    const share = result.share;

    // Check for duplicate index
    if (state.shares.some(s => s.index === share.index)) {
      errorHandlers.duplicateShare(share.index);
      return;
    }

    // Set threshold/total from first share
    if (state.shares.length === 0) {
      state.threshold = share.threshold;
      state.total = share.total;
    }

    state.shares.push(share);
    updateSharesUI();

    // If manifest is included and we don't have one yet, use it
    if (result.manifest && !state.manifest) {
      state.manifest = result.manifest;
      showManifestLoaded('MANIFEST.age', state.manifest.length, true);
    }

    checkRecoverReady();
  }

  async function parseAndAddShare(content, filename) {
    if (!state.wasmReady) {
      toast.warning(t('error_not_ready_title'), t('error_not_ready_message'), t('error_not_ready_guidance'));
      return;
    }

    // Check if content contains a share
    if (!shareRegex.test(content)) {
      errorHandlers.noShareFound(filename);
      return;
    }

    const result = window.rememoryParseShare(content);
    if (result.error) {
      errorHandlers.invalidShare(filename, result.error);
      return;
    }

    const share = result.share;

    // Check for duplicate index
    if (state.shares.some(s => s.index === share.index)) {
      errorHandlers.duplicateShare(share.index);
      return;
    }

    // Set threshold/total from first share
    if (state.shares.length === 0) {
      state.threshold = share.threshold;
      state.total = share.total;
    }

    state.shares.push(share);
    updateSharesUI();
    checkRecoverReady();
  }

  function updateSharesUI() {
    elements.sharesList.innerHTML = '';

    state.shares.forEach((share, idx) => {
      const item = document.createElement('div');
      item.className = 'share-item valid';

      // Check if this is the holder's own share (from personalization)
      const isHolderShare = share.isHolder ||
        (personalization && share.holder &&
         share.holder.toLowerCase() === personalization.holder.toLowerCase());

      const holderLabel = isHolderShare ? ` (${t('your_share')})` : '';
      const showRemove = !isHolderShare; // Don't allow removing holder's own share

      item.innerHTML = `
        <span class="icon">&#9989;</span>
        <div class="details">
          <div class="name">${escapeHtml(share.holder || 'Share ' + share.index)}${holderLabel}</div>
          <div class="meta">${t('share_index', share.index, share.total)}</div>
        </div>
        ${showRemove ? `<button class="remove" data-idx="${idx}" title="${t('remove')}">&times;</button>` : ''}
      `;
      elements.sharesList.appendChild(item);
    });

    // Add remove handlers
    elements.sharesList.querySelectorAll('.remove').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const idx = parseInt(e.target.dataset.idx, 10);
        state.shares.splice(idx, 1);
        if (state.shares.length === 0) {
          state.threshold = 0;
          state.total = 0;
        }
        updateSharesUI();
        updateContactList();
        checkRecoverReady();
      });
    });

    // Update threshold info
    if (state.threshold > 0) {
      const needed = Math.max(0, state.threshold - state.shares.length);
      elements.thresholdInfo.innerHTML = needed > 0
        ? `&#128274; ${t('need_more', needed)} (${t('shares_of', state.shares.length, state.threshold)})`
        : `&#9989; ${t('ready')} (${t('shares_of', state.shares.length, state.threshold)})`;
      elements.thresholdInfo.className = 'threshold-info' + (needed === 0 ? ' ready' : '');
      elements.thresholdInfo.classList.remove('hidden');
    } else {
      elements.thresholdInfo.classList.add('hidden');
    }

    // Update contact list checkboxes
    updateContactList();
  }

  // Handle manifest file upload
  async function handleManifestFiles(files) {
    if (files.length === 0) return;

    // Clear any previous inline errors
    clearInlineError(elements.manifestDropZone);

    try {
      const file = files[0];

      // Check if it's a ZIP file (bundle) - extract manifest and share from it
      if (file.name.endsWith('.zip') || file.type === 'application/zip') {
        await handleBundleZip(file);
        return;
      }

      // Validate file extension
      if (!file.name.endsWith('.age')) {
        showError(
          t('error_wrong_manifest_message', file.name),
          {
            title: t('error_wrong_manifest_title'),
            guidance: t('error_wrong_manifest_guidance'),
            inline: true,
            targetElement: elements.manifestDropZone
          }
        );
        return;
      }

      const buffer = await readFileAsArrayBuffer(file);
      state.manifest = new Uint8Array(buffer);

      showManifestLoaded(file.name, state.manifest.length);
      checkRecoverReady();
    } catch (err) {
      errorHandlers.fileReadFailed(files[0]?.name || 'file');
    }
  }

  // Show manifest loaded state with clear button
  function showManifestLoaded(filename, size, fromBundle = false) {
    elements.manifestDropZone.classList.add('hidden');
    elements.manifestStatus.innerHTML = `
      <span class="icon">&#9989;</span>
      <div style="flex: 1;">
        <strong>${escapeHtml(filename)}</strong> ${fromBundle ? t('manifest_loaded_bundle') : t('loaded')}
        <div style="font-size: 0.875rem; color: #6c757d;">${formatSize(size)}</div>
      </div>
      <button class="clear-manifest" title="${t('remove')}">&times;</button>
    `;
    elements.manifestStatus.classList.remove('hidden');
    elements.manifestStatus.classList.add('loaded');

    // Add clear handler
    elements.manifestStatus.querySelector('.clear-manifest').addEventListener('click', clearManifest);
  }

  // Clear manifest and show drop zone again
  function clearManifest() {
    state.manifest = null;
    elements.manifestStatus.classList.add('hidden');
    elements.manifestStatus.classList.remove('loaded');
    elements.manifestDropZone.classList.remove('hidden');
    checkRecoverReady();
  }

  // Setup buttons
  function setupButtons() {
    elements.recoverBtn.addEventListener('click', startRecovery);
    elements.downloadAllBtn.addEventListener('click', downloadAll);
  }

  function checkRecoverReady() {
    const ready = state.shares.length >= state.threshold &&
                  state.threshold > 0 &&
                  state.manifest !== null;
    elements.recoverBtn.disabled = !ready;

    // Auto-start recovery when conditions are met
    if (ready && !state.recovering && !state.recoveryComplete) {
      startRecovery();
    }
  }

  // Collapse steps 1 and 2 to focus on recovery
  function collapseInputSteps() {
    if (elements.step1Card) {
      elements.step1Card.classList.add('collapsed');
    }
    if (elements.step2Card) {
      elements.step2Card.classList.add('collapsed');
    }
  }

  // Recovery process
  async function startRecovery() {
    if (state.recovering) return;
    state.recovering = true;

    // Collapse steps 1 and 2 to focus on recovery
    collapseInputSteps();

    elements.recoverBtn.disabled = true;
    elements.progressBar.classList.remove('hidden');
    elements.statusMessage.className = 'status-message';
    elements.filesList.innerHTML = '';
    elements.downloadActions.classList.add('hidden');

    try {
      // Step 1: Combine shares
      setProgress(10);
      setStatus(t('combining'));

      const sharesForCombine = state.shares.map(s => ({
        index: s.index,
        dataB64: s.dataB64
      }));

      const combineResult = window.rememoryCombineShares(sharesForCombine);
      if (combineResult.error) {
        throw new Error(combineResult.error);
      }

      const passphrase = combineResult.passphrase;
      setProgress(30);

      // Step 2: Decrypt manifest
      setStatus(t('decrypting'));
      const decryptResult = window.rememoryDecryptManifest(state.manifest, passphrase);
      if (decryptResult.error) {
        throw new Error(decryptResult.error);
      }

      setProgress(60);

      // Store the decrypted tar.gz for download
      state.decryptedArchive = decryptResult.data;

      // Step 3: Extract tar.gz to show file list (preview only)
      setStatus(t('reading'));
      const extractResult = window.rememoryExtractTarGz(decryptResult.data);
      if (extractResult.error) {
        throw new Error(extractResult.error);
      }

      setProgress(90);

      // Step 4: Display files (preview)
      const files = extractResult.files;

      files.forEach(file => {
        const item = document.createElement('div');
        item.className = 'file-item';
        item.innerHTML = `
          <span class="icon">&#128196;</span>
          <span class="name">${escapeHtml(file.name)}</span>
          <span class="size">${formatSize(file.data.length)}</span>
        `;
        elements.filesList.appendChild(item);
      });

      setProgress(100);
      setStatus(t('complete', files.length), 'success');
      elements.downloadActions.classList.remove('hidden');
      elements.recoverBtn.classList.add('hidden');
      state.recoveryComplete = true;

    } catch (err) {
      const errorMsg = err.message || String(err);

      // Determine error type and show appropriate message
      if (errorMsg.includes('decrypt') || errorMsg.includes('passphrase') || errorMsg.includes('incorrect')) {
        errorHandlers.decryptionFailed(err);
        setStatus(t('error_decrypt_status'), 'error');
      } else if (errorMsg.includes('extract') || errorMsg.includes('tar') || errorMsg.includes('gzip')) {
        errorHandlers.extractionFailed(err);
        setStatus(t('error_extract_status'), 'error');
      } else {
        // Generic error with guidance
        toast.error(
          t('error_recovery_title'),
          errorMsg,
          t('error_recovery_guidance'),
          [
            { id: 'retry', label: t('action_try_again'), primary: true, onClick: () => startRecovery() }
          ]
        );
        setStatus(t('error', errorMsg), 'error');
      }

      // On error, show steps again so user can try different shares
      if (elements.step1Card) elements.step1Card.classList.remove('collapsed');
      if (elements.step2Card) elements.step2Card.classList.remove('collapsed');
    } finally {
      state.recovering = false;
      elements.recoverBtn.disabled = false;
    }
  }

  function setProgress(percent) {
    const fill = elements.progressBar.querySelector('.fill');
    fill.style.width = percent + '%';
  }

  function setStatus(msg, type) {
    elements.statusMessage.textContent = msg;
    elements.statusMessage.className = 'status-message' + (type ? ' ' + type : '');
  }

  // Download the decrypted archive
  function downloadAll() {
    if (!state.decryptedArchive) return;

    const blob = new Blob([state.decryptedArchive], { type: 'application/gzip' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'manifest.tar.gz';
    a.click();
    URL.revokeObjectURL(url);

    // Security: Clear sensitive data from memory after download
    clearSensitiveState();
  }

  // Clear sensitive data from state to minimize memory exposure
  function clearSensitiveState() {
    state.decryptedArchive = null;
    state.manifest = null;
    // Note: shares contain metadata but not the actual secret
  }

  // Utility functions
  function readFileAsText(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = reject;
      reader.readAsText(file);
    });
  }

  function readFileAsArrayBuffer(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = reject;
      reader.readAsArrayBuffer(file);
    });
  }


  // Error display with guidance - replaces alert()
  function showError(msg, options = {}) {
    const { title, guidance, actions, inline, targetElement } = options;

    // If inline error requested and target exists, show inline
    if (inline && targetElement) {
      showInlineError(targetElement, msg, guidance);
      return;
    }

    // Otherwise show toast
    toast.error(title || t('error_title'), msg, guidance, actions);
  }

  // Specific error handlers with guidance
  const errorHandlers = {
    wasmLoadFailed(err) {
      toast.error(
        t('error_wasm_title'),
        t('error_wasm_message'),
        t('error_wasm_guidance'),
        [
          { id: 'reload', label: t('action_reload'), primary: true, onClick: () => window.location.reload() },
          { id: 'cli', label: t('action_use_cli'), onClick: () => window.open('https://github.com/eljojo/rememory', '_blank') }
        ]
      );
    },

    invalidShare(filename, detail) {
      showError(
        t('error_invalid_share_message', filename),
        {
          title: t('error_invalid_share_title'),
          guidance: t('error_invalid_share_guidance'),
          inline: true,
          targetElement: elements.shareDropZone
        }
      );
    },

    noShareFound(filename) {
      showError(
        t('error_no_share_message', filename),
        {
          title: t('error_no_share_title'),
          guidance: t('error_no_share_guidance'),
          inline: true,
          targetElement: elements.shareDropZone
        }
      );
    },

    duplicateShare(index) {
      toast.warning(
        t('error_duplicate_title'),
        t('error_duplicate_message', index),
        t('error_duplicate_guidance')
      );
    },

    fileReadFailed(filename) {
      showError(
        t('error_file_read_message', filename),
        {
          title: t('error_file_read_title'),
          guidance: t('error_file_read_guidance')
        }
      );
    },

    decryptionFailed(err) {
      toast.error(
        t('error_decrypt_title'),
        t('error_decrypt_message'),
        t('error_decrypt_guidance'),
        [
          { id: 'retry', label: t('action_try_different_shares'), primary: true, onClick: () => {
            // Clear shares and let user try again
            state.shares = [];
            state.recoveryComplete = false;
            updateSharesUI();
            if (elements.step1Card) elements.step1Card.classList.remove('collapsed');
          }}
        ]
      );
    },

    extractionFailed(err) {
      toast.error(
        t('error_extract_title'),
        t('error_extract_message'),
        t('error_extract_guidance')
      );
    }
  };

  // Expose function to re-render UI when language changes
  window.rememoryUpdateUI = function() {
    updateSharesUI();
    updateContactList();
  };

  // Start
  document.addEventListener('DOMContentLoaded', init);
})();
