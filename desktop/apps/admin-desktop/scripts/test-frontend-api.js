const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

function testFrontendAPI() {
  return new Promise((resolve, reject) => {
    console.log('🧪 Testing Frontend API Connection');
    console.log('==================================');
    console.log('This will launch the app and test the API connection from the frontend');
    console.log('');
    
    const appPath = path.join(__dirname, '..', 'release', 'mac-arm64', 'FamilyVault.app');
    
    if (!fs.existsSync(appPath)) {
      reject(new Error('Packaged app not found. Run pnpm electron:build first.'));
      return;
    }
    
    console.log('📋 Manual Testing Instructions:');
    console.log('==============================');
    console.log('1. The app will open with DevTools enabled');
    console.log('2. Navigate to the "Create Family Group" page');
    console.log('3. Fill in the form and click "Create Group"');
    console.log('4. Check the DevTools Console tab for any errors');
    console.log('5. Check the DevTools Network tab for the API request');
    console.log('');
    console.log('🔍 What to look for:');
    console.log('- Is the request being made to http://127.0.0.1:8000/groups?');
    console.log('- What is the request payload?');
    console.log('- What is the response status and body?');
    console.log('- Are there any CORS errors?');
    console.log('- Are there any JavaScript errors?');
    console.log('');
    
    // Launch the app
    const child = spawn('open', ['-W', appPath], {
      stdio: ['ignore', 'pipe', 'pipe']
    });
    
    child.stdout.on('data', (data) => {
      console.log(`[APP] ${data.toString().trim()}`);
    });
    
    child.stderr.on('data', (data) => {
      console.log(`[APP-ERR] ${data.toString().trim()}`);
    });
    
    setTimeout(() => {
      console.log('⏰ Test window opened. Please test the group creation manually.');
      console.log('');
      console.log('💡 Common Issues to Check:');
      console.log('==========================');
      console.log('1. Network Request Fails:');
      console.log('   - Backend not running (check if http://127.0.0.1:8000/health works)');
      console.log('   - CORS issues (check browser console)');
      console.log('   - Wrong request format');
      console.log('');
      console.log('2. Request Format Issues:');
      console.log('   - Missing required fields (name, owner_display_name)');
      console.log('   - Wrong Content-Type header');
      console.log('   - Invalid JSON payload');
      console.log('');
      console.log('3. Authentication Issues:');
      console.log('   - Token storage problems');
      console.log('   - Token not being sent with request');
      console.log('');
      resolve();
    }, 2000);
    
    child.on('error', (error) => {
      reject(error);
    });
  });
}

async function main() {
  try {
    await testFrontendAPI();
  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { testFrontendAPI };