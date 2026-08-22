// Code.gs

// ===== CONFIGURATION =====
const DRIVE_FOLDER_ID = '<folder_id>';   // From the Drive folder URL
const STORAGE_BUCKET  = '<project>-receipts-unprocessed';

/**
 * Main function — checks the folder for new files and uploads them.
 * Run this on a time-based trigger (e.g., every 10 minutes).
 */
function checkAndUploadNewFiles() {
  const folder = DriveApp.getFolderById(DRIVE_FOLDER_ID);
  const files = folder.getFiles();

  // Load the set of already-processed file IDs from persistent storage
  const props = PropertiesService.getScriptProperties();
  const processedRaw = props.getProperty('processed_files') || '[]';
  const processed = JSON.parse(processedRaw);
  const processedSet = new Set(processed);

  const newProcessed = [...processed];

  while (files.hasNext()) {
  const file = files.next();
  const fileId = file.getId();
  Logger.log(`File found ${fileId}`)

  if (!processedSet.has(fileId)) {
    const ok = uploadFileToCloudStorage(file);
    if (ok) {
      processedSet.add(fileId);
      newProcessed.push(fileId);
    }
    // If upload failed, do NOT mark as processed — it will be retried next run
  }
}

  // Persist the updated list
  props.setProperty('processed_files', JSON.stringify(newProcessed));
}

/**
 * Uploads a single Drive file to Google Cloud Storage,
 * then deletes it from Drive ONLY if the upload succeeded.
 * @param {GoogleAppsScript.Drive.File} file
 * @returns {boolean} true if the upload succeeded
 */
function uploadFileToCloudStorage(file) {
  const blob = file.getBlob();
  const bytes = blob.getBytes();

  const API = 'https://www.googleapis.com/upload/storage/v1/b';
  const location = encodeURIComponent(file.getName());
  const url = `${API}/${STORAGE_BUCKET}/o?uploadType=media&name=${location}`;

  const service = getStorageService();
  const accessToken = service.getAccessToken();

  const response = UrlFetchApp.fetch(url, {
    method: 'POST',
    contentLength: bytes.length,
    contentType: blob.getContentType(),
    payload: bytes,
    headers: {
      Authorization: `Bearer ${accessToken}`
    }
  });

  // Confirm success before doing anything destructive
  const status = response.getResponseCode();
  if (status !== 200 && status !== 201) {
    Logger.log(`Upload FAILED (HTTP ${status}) for ${file.getName()} — file NOT deleted`);
    return false;
  }

  const result = JSON.parse(response.getContentText());
  if (!result.id) {
    Logger.log(`Upload returned no object id for ${file.getName()} — file NOT deleted`);
    return false;
  }

  Logger.log(`Uploaded: ${result.name} (size: ${result.size} bytes)`);

  // ===== SUCCESS — now safe to delete from Drive =====
  file.setTrashed(true);          // moves to Trash (recoverable)

  Logger.log(`Moved to Trash: ${file.getName()}`);
  return true;
}

