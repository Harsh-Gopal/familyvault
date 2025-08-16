const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

function findPackagedApp() {
  const releaseDir = path.join(__dirname, '..', 'release');
  const macDir = path.join(releaseDir, 'mac-arm64');
  
  if (fs.existsSync(macDir)) {
    const files = fs.readdirSync(macDir);
    const appFile = files.find(file => file.endsWith('.app'));
    if (appFile) {
      return path.join(macDir, appFile);
    }
  }
  
  throw new Error('Packaged app not found');
}

function testUIRendering(appPath, timeoutMs = 15000) {
  return new Promise((resolve, reject) => {
    console.log('🧪 Testing UI Rendering in Packaged App');
    console.log('=======================================');
    console.log(`App: ${path.basename(appPath)}`);
    console.log(`Timeout: ${timeoutMs}ms`);
    console.log('');
    console.log('📋 What to check when the app opens:');
    console.log('1. Does the window show content or is it blank/white?');
    console.log('2. Are there any console errors in DevTools?');
    console.log('3. Do the React components render properly?');
    console.log('4. Are CSS styles applied correctly?');
    console.log('');
    console.log('🔧 DevTools should open automatically for debugging');
    console.log('');
    
    // Launch the app with console output
    const child = spawn('open', ['-W', appPath], {
      stdio: ['ignore', 'pipe', 'pipe']
    });
    
    let stdout = '';
    let stderr = '';
    
    child.stdout.on('data', (data) => {
      const output = data.toString();
      stdout += output;
      console.log(`[APP-STDOUT] ${output.trim()}`);
    });
    
    child.stderr.on('data', (data) => {
      const output = data.toString();
      stderr += output;
      console.log(`[APP-STDERR] ${output.trim()}`);
    });
    
    const timeout = setTimeout(() => {
      console.log('⏰ Test completed - app should be running');
      console.log('');
      console.log('📊 Manual Verification Checklist:');
      console.log('=================================');
      console.log('□ App window opened');
      console.log('□ UI content is visible (not white screen)');
      console.log('□ DevTools opened automatically');
      console.log('□ No console errors in DevTools');
      console.log('□ React components rendered');
      console.log('□ CSS styles applied');
      console.log('□ Navigation works');
      console.log('');
      console.log('If you see a white screen:');
      console.log('1. Check DevTools Console tab for JavaScript errors');
      console.log('2. Check DevTools Network tab for failed resource loads');
      console.log('3. Check DevTools Sources tab to see if files are loaded');
      console.log('');
      
      resolve({ stdout, stderr });
    }, timeoutMs);
    
    child.on('exit', (code) => {
      clearTimeout(timeout);
      console.log(`App process exited with code: ${code}`);
      resolve({ stdout, stderr, exitCode: code });
    });
    
    child.on('error', (error) => {
      clearTimeout(timeout);
      reject(error);
    });
  });
}

async function main() {
  try {
    const appPath = findPackagedApp();
    await testUIRendering(appPath);
  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { testUIRendering, findPackagedApp };