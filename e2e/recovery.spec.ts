import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import {
  getRememoryBin,
  createTestProject,
  extractBundle,
  extractBundles,
  RecoveryPage
} from './helpers';

test.describe('Browser Recovery Tool', () => {
  let projectDir: string;
  let bundlesDir: string;

  test.beforeAll(async () => {
    // Skip if rememory binary not available
    const bin = getRememoryBin();
    if (!fs.existsSync(bin)) {
      console.log(`Skipping tests: rememory binary not found at ${bin}`);
      test.skip();
      return;
    }

    projectDir = createTestProject();
    bundlesDir = path.join(projectDir, 'output', 'bundles');
  });

  test.afterAll(async () => {
    if (projectDir && fs.existsSync(projectDir)) {
      fs.rmSync(projectDir, { recursive: true, force: true });
    }
  });

  test('recover.html loads and shows UI', async ({ page }) => {
    const bundleDir = extractBundle(bundlesDir, 'Alice');
    const recovery = new RecoveryPage(page, bundleDir);

    await recovery.open();
    await recovery.expectUIElements();
  });

  test('personalized recover.html pre-loads holder share', async ({ page }) => {
    const bundleDir = extractBundle(bundlesDir, 'Alice');
    const recovery = new RecoveryPage(page, bundleDir);

    await recovery.open();

    // Alice's share should already be loaded (personalization)
    await recovery.expectShareCount(1);
    await recovery.expectShareHolder('Alice');
    await recovery.expectHolderShareLabel();

    // Manifest is NOT pre-loaded - user must load it
    await recovery.expectManifestDropZoneVisible();

    // Still need 1 more share (threshold is 2)
    await recovery.expectNeedMoreShares(1);
  });

  test('shows contact list for other friends', async ({ page }) => {
    const bundleDir = extractBundle(bundlesDir, 'Alice');
    const recovery = new RecoveryPage(page, bundleDir);

    await recovery.open();

    // Contact list should show Bob and Carol (other friends)
    await recovery.expectContactListVisible();
    await recovery.expectContactItem('Bob');
    await recovery.expectContactItem('Carol');
  });

  test('contact list updates when shares are collected', async ({ page }) => {
    const [aliceDir, bobDir] = extractBundles(bundlesDir, ['Alice', 'Bob']);
    const recovery = new RecoveryPage(page, aliceDir);

    await recovery.open();

    // Bob's contact should not be checked initially
    await recovery.expectContactNotCollected('Bob');

    // Add Bob's share
    await recovery.addShares(bobDir);

    // Bob's contact should now be checked
    await recovery.expectContactCollected('Bob');
  });

  test('paste share functionality', async ({ page }) => {
    const [aliceDir, bobDir] = extractBundles(bundlesDir, ['Alice', 'Bob']);
    const recovery = new RecoveryPage(page, aliceDir);

    await recovery.open();

    // Click paste button to show textarea
    await recovery.clickPasteButton();
    await recovery.expectPasteAreaVisible();

    // Read Bob's share and paste it
    const bobShare = fs.readFileSync(path.join(bobDir, 'README.txt'), 'utf8');
    await recovery.pasteShare(bobShare);
    await recovery.submitPaste();

    // Bob's share should be added
    await recovery.expectShareCount(2);
    await recovery.expectShareHolder('Bob');
  });

  test('auto-recovery when threshold is met', async ({ page }) => {
    const [aliceDir, bobDir] = extractBundles(bundlesDir, ['Alice', 'Bob']);
    const recovery = new RecoveryPage(page, aliceDir);

    await recovery.open();

    // Alice's share is pre-loaded
    await recovery.expectShareCount(1);

    // Load manifest first
    await recovery.addManifest();
    await recovery.expectManifestLoaded();

    // Add Bob's share - should trigger auto-recovery
    await recovery.addShares(bobDir);

    // Recovery should complete automatically
    await recovery.expectRecoveryComplete();
    await recovery.expectFileCount(3);
    await recovery.expectDownloadVisible();
  });

  test('steps collapse after recovery starts', async ({ page }) => {
    const [aliceDir, bobDir] = extractBundles(bundlesDir, ['Alice', 'Bob']);
    const recovery = new RecoveryPage(page, aliceDir);

    await recovery.open();

    // Steps 1 and 2 should be visible initially
    await recovery.expectStepsVisible();

    // Load manifest first
    await recovery.addManifest();

    // Add Bob's share - triggers auto-recovery
    await recovery.addShares(bobDir);

    // Steps should collapse
    await recovery.expectStepsCollapsed();
  });

  test('can add shares from README.txt files', async ({ page }) => {
    const [aliceDir, bobDir] = extractBundles(bundlesDir, ['Alice', 'Bob']);
    const recovery = new RecoveryPage(page, aliceDir);

    await recovery.open();

    // Alice's share is already pre-loaded via personalization
    await recovery.expectShareCount(1);
    await recovery.expectShareHolder('Alice');

    // Add Bob's share
    await recovery.addShares(bobDir);
    await recovery.expectShareCount(2);
    await recovery.expectReadyToRecover();
  });

  test('full recovery workflow', async ({ page }) => {
    const [aliceDir, bobDir] = extractBundles(bundlesDir, ['Alice', 'Bob']);
    const recovery = new RecoveryPage(page, aliceDir);

    await recovery.open();

    // Alice's share is pre-loaded via personalization
    await recovery.expectShareCount(1);

    // Load manifest
    await recovery.addManifest();
    await recovery.expectManifestLoaded();

    // Add Bob's share (triggers auto-recovery)
    await recovery.addShares(bobDir);

    // Recovery should complete automatically
    await recovery.expectRecoveryComplete();
    await recovery.expectFileCount(3); // secret.txt, notes.txt, README.md
    await recovery.expectDownloadVisible();
  });

  test('shows need for more shares with only holder share', async ({ page }) => {
    const bundleDir = extractBundle(bundlesDir, 'Alice');
    const recovery = new RecoveryPage(page, bundleDir);

    await recovery.open();

    // Only holder's share is loaded (threshold is 2)
    await recovery.expectShareCount(1);
    await recovery.expectNeedMoreShares(1);
  });

  test('detects duplicate shares', async ({ page }) => {
    const bundleDir = extractBundle(bundlesDir, 'Alice');
    const recovery = new RecoveryPage(page, bundleDir);

    await recovery.open();
    recovery.onDialog('dismiss');

    // Alice's share is already pre-loaded, try to add it again
    await recovery.addShares(bundleDir);
    await recovery.expectShareCount(1); // Still 1, duplicate ignored
  });
});
