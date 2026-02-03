import { Page, expect } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import AdmZip from 'adm-zip';

// Get absolute path to rememory binary
export function getRememoryBin(): string {
  const binEnv = process.env.REMEMORY_BIN || './rememory';
  return path.resolve(binEnv);
}

// Create a sealed test project with bundles
export function createTestProject(): string {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rememory-e2e-'));
  const projectDir = path.join(tmpDir, 'test-project');
  const bin = getRememoryBin();

  // Create project with 3 friends, threshold 2
  execSync(`${bin} init ${projectDir} --name "E2E Test" --threshold 2 --friend "Alice,alice@test.com" --friend "Bob,bob@test.com" --friend "Carol,carol@test.com"`, {
    stdio: 'inherit'
  });

  // Add secret content
  const manifestDir = path.join(projectDir, 'manifest');
  fs.writeFileSync(path.join(manifestDir, 'secret.txt'), 'The secret password is: correct-horse-battery-staple');
  fs.writeFileSync(path.join(manifestDir, 'notes.txt'), 'Remember to feed the cat!');

  // Seal and generate bundles
  execSync(`${bin} seal`, { cwd: projectDir, stdio: 'inherit' });
  execSync(`${bin} bundle`, { cwd: projectDir, stdio: 'inherit' });

  return projectDir;
}

// Extract a bundle ZIP and return the extracted directory path
// Note: friendName is case-insensitive, bundle files are lowercase
export function extractBundle(bundlesDir: string, friendName: string): string {
  const lowerName = friendName.toLowerCase();
  const bundleZip = path.join(bundlesDir, `bundle-${lowerName}.zip`);
  const extractDir = path.join(bundlesDir, `bundle-${lowerName}`);

  fs.mkdirSync(extractDir, { recursive: true });

  // Use adm-zip for cross-platform extraction
  const zip = new AdmZip(bundleZip);
  zip.extractAllTo(extractDir, true);

  return extractDir;
}

// Extract multiple bundles
export function extractBundles(bundlesDir: string, friendNames: string[]): string[] {
  return friendNames.map(name => extractBundle(bundlesDir, name));
}

// Page helper class for recovery tool interactions
export class RecoveryPage {
  constructor(private page: Page, private bundleDir: string) {}

  // Navigate to recover.html and wait for WASM
  async open(): Promise<void> {
    await this.page.goto(`file://${path.join(this.bundleDir, 'recover.html')}`);
    await this.page.waitForFunction(
      () => (window as any).rememoryAppReady === true,
      { timeout: 30000 }
    );
  }

  // Add shares from README.txt files
  async addShares(...bundleDirs: string[]): Promise<void> {
    const readmePaths = bundleDirs.map(dir => path.join(dir, 'README.txt'));
    await this.page.locator('#share-file-input').setInputFiles(readmePaths);
  }

  // Add manifest file
  async addManifest(bundleDir?: string): Promise<void> {
    const dir = bundleDir || this.bundleDir;
    await this.page.locator('#manifest-file-input').setInputFiles(
      path.join(dir, 'MANIFEST.age')
    );
  }

  // Click recover button
  async recover(): Promise<void> {
    await this.page.locator('#recover-btn').click();
  }

  // Assertions
  async expectShareCount(count: number): Promise<void> {
    await expect(this.page.locator('.share-item')).toHaveCount(count);
  }

  async expectShareHolder(name: string): Promise<void> {
    await expect(this.page.locator('.share-item').filter({ hasText: name })).toBeVisible();
  }

  async expectReadyToRecover(): Promise<void> {
    await expect(this.page.locator('#threshold-info')).toHaveClass(/ready/);
  }

  async expectNeedMoreShares(count: number): Promise<void> {
    await expect(this.page.locator('#threshold-info')).toContainText(`Waiting for ${count} more piece`);
  }

  async expectManifestLoaded(): Promise<void> {
    await expect(this.page.locator('#manifest-status')).toHaveClass(/loaded/);
  }

  async expectManifestDropZoneVisible(): Promise<void> {
    await expect(this.page.locator('#manifest-drop-zone')).toBeVisible();
  }

  async expectRecoverEnabled(): Promise<void> {
    await expect(this.page.locator('#recover-btn')).toBeEnabled();
  }

  async expectRecoverDisabled(): Promise<void> {
    await expect(this.page.locator('#recover-btn')).toBeDisabled();
  }

  async expectRecoveryComplete(): Promise<void> {
    await expect(this.page.locator('#status-message')).toContainText('All done', { timeout: 60000 });
  }

  async expectFileCount(count: number): Promise<void> {
    await expect(this.page.locator('.file-item')).toHaveCount(count);
  }

  async expectDownloadVisible(): Promise<void> {
    await expect(this.page.locator('#download-all-btn')).toBeVisible();
  }

  async expectUIElements(): Promise<void> {
    await expect(this.page.locator('h1')).toContainText('🧠 ReMemory Recovery');
    await expect(this.page.locator('#share-drop-zone')).toBeVisible();
    await expect(this.page.locator('#manifest-drop-zone')).toBeVisible();
  }

  // Dismiss dialogs (for duplicate share tests)
  onDialog(action: 'dismiss' | 'accept' = 'dismiss'): void {
    this.page.on('dialog', dialog => dialog[action]());
  }

  // Paste functionality
  async clickPasteButton(): Promise<void> {
    await this.page.locator('#paste-toggle-btn').click();
  }

  async expectPasteAreaVisible(): Promise<void> {
    await expect(this.page.locator('#paste-area')).toBeVisible();
  }

  async pasteShare(content: string): Promise<void> {
    await this.page.locator('#paste-input').fill(content);
  }

  async submitPaste(): Promise<void> {
    await this.page.locator('#paste-submit-btn').click();
  }

  // Holder share label check
  async expectHolderShareLabel(): Promise<void> {
    await expect(this.page.locator('.share-item').first()).toContainText('Your piece');
  }

  // Contact list assertions
  async expectContactListVisible(): Promise<void> {
    await expect(this.page.locator('#contact-list-section')).toBeVisible();
  }

  async expectContactItem(name: string): Promise<void> {
    await expect(this.page.locator('.contact-item').filter({ hasText: name })).toBeVisible();
  }

  async expectContactCollected(name: string): Promise<void> {
    const contact = this.page.locator('.contact-item').filter({ hasText: name });
    await expect(contact).toHaveClass(/collected/);
  }

  async expectContactNotCollected(name: string): Promise<void> {
    const contact = this.page.locator('.contact-item').filter({ hasText: name });
    await expect(contact).not.toHaveClass(/collected/);
  }

  // Steps collapse assertions
  async expectStepsVisible(): Promise<void> {
    await expect(this.page.locator('.card').first()).toBeVisible();
  }

  async expectStepsCollapsed(): Promise<void> {
    await expect(this.page.locator('.card.collapsed').first()).toBeAttached();
  }
}
