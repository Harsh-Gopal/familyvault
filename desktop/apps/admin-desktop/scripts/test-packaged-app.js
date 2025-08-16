const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

function findPackagedApp() {
  const releaseDir = path.join(__dirname, '..', 'release');
  
  if (!fs.existsSync(releaseDir)) {
    throw new Error('Release directory not found. Run pnpm electron:build first.');
  }
  
  // Look for the .app bundle in mac-arm64 directory
  const macDir = path.join(releaseDir, 'mac-arm64');
  if (fs.existsSync(macDir)) {
    const files = fs.readdirSync(macDir);
    const appFile = files.find(file => file.endsWith('.app'));
    if (appFile) {
      return path.join(macDir, appFile);
    }
  }
  
  throw new Error('Packaged app not found in release directory');
}

function testPackagedApp(appPath, timeoutMs = 30000) {
  return new Promise((resolve, reject) => {
    console.log(`🚀 Testing packaged app: ${path.basename(appPath)}`);
    console.log(`   Path: ${appPath}`);
    console.log(`   Timeout: ${timeoutMs}ms`);
    console.log('');
    
    // Launch the app
    const child = spawn('open', ['-W', '-n', appPath], {
      stdio: ['ignore', 'pipe', 'pipe']
    });
    
    let stdout = '';
    let stderr = '';
    let hasErrors = false;
    
    // Capture stdout
    child.stdout.on('data', (data) => {
      const output = data.toString();
      stdout += output;
      console.log(`[STDOUT] ${output.trim()}`);
    });
    
    // Capture stderr and check for module errors
    child.stderr.on('data', (data) => {
      const output = data.toString();
      stderr += output;
      console.log(`[STDERR] ${output.trim()}`);
      
      // Check for common module loading errors
      if (output.includes('Cannot find module') || 
          output.includes('MODULE_NOT_FOUND') ||
          output.includes('Error: Cannot resolve module')) {
        hasErrors = true;
        console.log('❌ Module loading error detected!');
      }
    });
    
    // Set timeout
    const timeout = setTimeout(() => {
      console.log('⏰ Test timeout reached');
      child.kill('SIGTERM');
      
      if (hasErrors) {
        reject(new Error('Module loading errors detected in packaged app'));
      } else {
        console.log('✅ No module errors detected during startup');
        resolve({ stdout, stderr, hasErrors: false });
      }
    }, timeoutMs);
    
    // Handle process exit
    child.on('exit', (code, signal) => {
      clearTimeout(timeout);
      
      console.log(`📊 App process exited with code: ${code}, signal: ${signal}`);
      
      if (hasErrors) {
        reject(new Error('Module loading errors detected in packaged app'));
      } else if (code !== 0 && code !== null) {
        reject(new Error(`App exited with non-zero code: ${code}`));
      } else {
        console.log('✅ App launched successfully without module errors');
        resolve({ stdout, stderr, hasErrors: false });
      }
    });
    
    child.on('error', (error) => {
      clearTimeout(timeout);
      reject(new Error(`Failed to launch app: ${error.message}`));
    });
  });
}

async function main() {
  console.log('🧪 Packaged App Runtime Test');
  console.log('============================');
  
  try {
    const appPath = findPackagedApp();
    await testPackagedApp(appPath);
    
    console.log('');
    console.log('🎉 SUCCESS: Packaged app launched without module errors!');
    process.exit(0);
    
  } catch (error) {
    console.log('');
    console.log('❌ FAILURE:', error.message);
    console.log('');
    console.log('💡 Troubleshooting:');
    console.log('   1. Check that all required modules are in package.json dependencies');
    console.log('   2. Verify asarUnpack configuration includes all necessary modules');
    console.log('   3. Run the dependency checker: node scripts/check-runtime-deps.js');
    console.log('   4. Rebuild the app: pnpm build && pnpm electron:build');
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { findPackagedApp, testPackagedApp };