const getStorageService = () => {
  const props = PropertiesService.getScriptProperties();
  const clientEmail = props.getProperty('GCS_CLIENT_EMAIL');
  const privateKey  = props.getProperty('GCS_PRIVATE_KEY');

  return OAuth2.createService('CloudStorage')
    .setPrivateKey(privateKey)
    .setIssuer(clientEmail)
    .setPropertyStore(PropertiesService.getUserProperties())
    .setCache(CacheService.getUserCache())
    .setTokenUrl('https://oauth2.googleapis.com/token')
    .setScope('https://www.googleapis.com/auth/devstorage.read_write');
};

function verifyCredentials() {
  const props = PropertiesService.getScriptProperties();
  const email = props.getProperty('GCS_CLIENT_EMAIL');
  const key   = props.getProperty('GCS_PRIVATE_KEY');
  // Verify immediately, same store:
  console.log('Client email:', props.getProperty('GCS_CLIENT_EMAIL'));
  console.log('Key stored:', Boolean(props.getProperty('GCS_PRIVATE_KEY')));
  console.log('Key length:', props.getProperty('GCS_PRIVATE_KEY').length);         // ✅ console.log handles multiple args
}


function setupCredentials() {
  const props = PropertiesService.getScriptProperties();
  props.setProperty('GCS_CLIENT_EMAIL', 'gdrive-uploader-sa@<project>.iam.gserviceaccount.com')
  props.setProperty('GCS_PRIVATE_KEY', `-----BEGIN PRIVATE KEY----- -----END PRIVATE KEY-----
`);
 verifyCredentials()
}