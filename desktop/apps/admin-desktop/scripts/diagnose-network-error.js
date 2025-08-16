const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const axios = require('axios');

const BACKEND_PORT = 8000;
const BACKEND_URL = `http://127.0.0.1:${BACKEND_PORT}`;

function checkBackendBinary() {
  console.log('🔍 Checking Backend Binary');
  console.log('==========================');
  
  const devBackendPath = path.join(__dirname, '..', 'build', 'backend', 'familyvault');
  const prodBackendPath = path.join(process.resourcesPath || '', 'app', 'build', 'backend', 'familyvault');
  
  console.log(`Dev backend path: ${devBackendPath}`);
  console.log(`Prod backend path: ${prodBackendPath}`);
  
  if (fs.existsSync(devBackendPath)) {
    console.log('✅ Development backend binary exists');
    
    // Check if it's executable
    try {
      fs.accessSync(devBackendPath, fs.constants.X_OK);
      console.log('✅ Backend binary is executable');
    } catch (error) {
      console.log('❌ Backend binary is not executable');
      console.log('   Run: chmod +x build/backend/familyvault');
    }
    
    return devBackendPath;
  } else {
    console.log('❌ Development backend binary missing');
  }
  
  if (fs.existsSync(prodBackendPath)) {
    console.log('✅ Production backend binary exists');
    return prodBackendPath;
  } else {
    console.log('❌ Production backend binary missing');
  }
  
  return null;
}

async function testBackendConnection() {
  console.log('\n🌐 Testing Backend Connection');
  console.log('=============================');
  
  try {
    console.log(`Attempting to connect to: ${BACKEND_URL}/health`);
    const response = await axios.get(`${BACKEND_URL}/health`, { timeout: 5000 });
    console.log('✅ Backend is running and responding');
    console.log(`   Status: ${response.status}`);
    console.log(`   Data:`, response.data);
    return true;
  } catch (error) {
    console.log('❌ Backend connection failed');
    if (error.code === 'ECONNREFUSED') {
      console.log('   Error: Connection refused - backend is not running');
    } else if (error.code === 'ETIMEDOUT') {
      console.log('   Error: Connection timeout - backend may be starting');
    } else {
      console.log(`   Error: ${error.message}`);
    }
    return false;
  }
}

function startBackendManually(backendPath) {
  return new Promise((resolve, reject) => {
    console.log('\n🚀 Starting Backend Manually');
    console.log('============================');
    
    if (!backendPath) {
      reject(new Error('Backend binary not found'));
      return;
    }
    
    console.log(`Starting: ${backendPath}`);
    
    // Set up environment
    const env = {
      ...process.env,
      FAMILYVAULT_DRIVE_PATH: path.join(require('os').homedir(), 'FamilyVault'),
      FAMILYVAULT_DATA_PATH: path.join(require('os').homedir(), '.familyvault'),
      PORT: BACKEND_PORT.toString(),
    };
    
    // Ensure directories exist
    fs.mkdirSync(env.FAMILYVAULT_DRIVE_PATH, { recursive: true });
    fs.mkdirSync(env.FAMILYVAULT_DATA_PATH, { recursive: true });
    
    const backendProcess = spawn(backendPath, [], {
      env,
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: false,
    });
    
    let startupOutput = '';
    let errorOutput = '';
    
    backendProcess.stdout.on('data', (data) => {
      const output = data.toString();
      startupOutput += output;
      console.log(`[BACKEND] ${output.trim()}`);
    });
    
    backendProcess.stderr.on('data', (data) => {
      const output = data.toString();
      errorOutput += output;
      console.log(`[BACKEND-ERR] ${output.trim()}`);
    });
    
    backendProcess.on('error', (error) => {
      console.log(`❌ Failed to start backend: ${error.message}`);
      reject(error);
    });
    
    backendProcess.on('exit', (code) => {
      console.log(`Backend process exited with code: ${code}`);
      if (code !== 0) {
        reject(new Error(`Backend exited with code ${code}`));
      }
    });
    
    // Wait a bit for startup
    setTimeout(async () => {
      const isRunning = await testBackendConnection();
      if (isRunning) {
        console.log('✅ Backend started successfully');
        resolve(backendProcess);
      } else {
        console.log('❌ Backend failed to start properly');
        console.log('Startup output:', startupOutput);
        console.log('Error output:', errorOutput);
        reject(new Error('Backend failed to start'));
      }
    }, 3000);
  });
}

async function testAPIEndpoints() {
  console.log('\n🧪 Testing API Endpoints');
  console.log('========================');
  
  const endpoints = [
    { path: '/health', method: 'GET', description: 'Health check' },
    { path: '/api/groups', method: 'GET', description: 'List groups' },
    { path: '/api/groups', method: 'POST', description: 'Create group', 
      data: { name: 'Test Group', admin_name: 'Test Admin' } }
  ];
  
  for (const endpoint of endpoints) {
    try {
      console.log(`Testing ${endpoint.method} ${endpoint.path} - ${endpoint.description}`);
      
      let response;
      if (endpoint.method === 'GET') {
        response = await axios.get(`${BACKEND_URL}${endpoint.path}`, { timeout: 5000 });
      } else if (endpoint.method === 'POST') {
        response = await axios.post(`${BACKEND_URL}${endpoint.path}`, endpoint.data, { timeout: 5000 });
      }
      
      console.log(`   ✅ Status: ${response.status}`);
      console.log(`   📄 Response:`, JSON.stringify(response.data, null, 2));
      
    } catch (error) {
      console.log(`   ❌ Failed: ${error.message}`);
      if (error.response) {
        console.log(`   📄 Error Response:`, error.response.data);
      }
    }
  }
}

async function checkNetworkConfiguration() {
  console.log('\n🔧 Network Configuration Check');
  console.log('==============================');
  
  // Check if port is in use
  const net = require('net');
  const server = net.createServer();
  
  return new Promise((resolve) => {
    server.listen(BACKEND_PORT, '127.0.0.1', () => {
      console.log(`✅ Port ${BACKEND_PORT} is available`);
      server.close();
      resolve(true);
    });
    
    server.on('error', (err) => {
      if (err.code === 'EADDRINUSE') {
        console.log(`⚠️  Port ${BACKEND_PORT} is already in use`);
        console.log('   This might mean the backend is already running');
      } else {
        console.log(`❌ Port check failed: ${err.message}`);
      }
      resolve(false);
    });
  });
}

async function main() {
  console.log('🔍 NETWORK ERROR DIAGNOSTIC');
  console.log('===========================');
  console.log('This script will help diagnose the "Network Error" issue');
  console.log('');
  
  try {
    // Step 1: Check backend binary
    const backendPath = checkBackendBinary();
    
    // Step 2: Check network configuration
    await checkNetworkConfiguration();
    
    // Step 3: Test if backend is already running
    const isAlreadyRunning = await testBackendConnection();
    
    if (!isAlreadyRunning && backendPath) {
      // Step 4: Try to start backend manually
      try {
        await startBackendManually(backendPath);
        
        // Step 5: Test API endpoints
        await testAPIEndpoints();
        
      } catch (error) {
        console.log(`❌ Failed to start backend: ${error.message}`);
      }
    } else if (isAlreadyRunning) {
      // Step 5: Test API endpoints
      await testAPIEndpoints();
    }
    
    console.log('\n💡 TROUBLESHOOTING GUIDE');
    console.log('========================');
    console.log('If you see "Network Error" in the app:');
    console.log('');
    console.log('1. Backend Not Running:');
    console.log('   - Check if backend binary exists and is executable');
    console.log('   - Ensure port 8000 is not blocked by firewall');
    console.log('   - Check backend logs for startup errors');
    console.log('');
    console.log('2. Frontend Connection Issues:');
    console.log('   - Verify frontend is making requests to http://127.0.0.1:8000');
    console.log('   - Check browser DevTools Network tab for failed requests');
    console.log('   - Ensure CORS is properly configured in backend');
    console.log('');
    console.log('3. API Endpoint Issues:');
    console.log('   - Verify the group creation endpoint exists');
    console.log('   - Check request payload format');
    console.log('   - Review backend API documentation');
    
  } catch (error) {
    console.error('❌ Diagnostic failed:', error.message);
  }
}

if (require.main === module) {
  main();
}

module.exports = { main, testBackendConnection, checkBackendBinary };