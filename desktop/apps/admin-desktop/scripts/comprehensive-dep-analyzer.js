const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Built-in Node.js modules
const BUILTIN_MODULES = new Set([
  'assert', 'buffer', 'child_process', 'cluster', 'crypto', 'dgram', 'dns',
  'domain', 'events', 'fs', 'http', 'https', 'net', 'os', 'path', 'punycode',
  'querystring', 'readline', 'stream', 'string_decoder', 'timers', 'tls',
  'tty', 'url', 'util', 'v8', 'vm', 'zlib', 'constants', 'module',
  'process', 'console', 'inspector', 'perf_hooks', 'trace_events',
  'worker_threads', 'async_hooks', 'http2', 'repl', 'wasi'
]);

// Electron modules
const ELECTRON_MODULES = new Set([
  'electron', 'electron/main', 'electron/renderer', 'electron/common'
]);

function getAllDependenciesRecursive(moduleName, nodeModulesPath, visited = new Set(), depth = 0) {
  const maxDepth = 10; // Prevent infinite recursion
  if (visited.has(moduleName) || depth > maxDepth) {
    return new Set();
  }
  
  visited.add(moduleName);
  const allDeps = new Set();
  
  const modulePath = path.join(nodeModulesPath, moduleName);
  const packageJsonPath = path.join(modulePath, 'package.json');
  
  if (!fs.existsSync(packageJsonPath)) {
    console.log(`${'  '.repeat(depth)}❌ Package not found: ${moduleName}`);
    return allDeps;
  }
  
  try {
    const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
    const dependencies = {
      ...packageJson.dependencies,
      ...packageJson.peerDependencies,
      ...packageJson.optionalDependencies
    };
    
    console.log(`${'  '.repeat(depth)}📦 ${moduleName} (${Object.keys(dependencies).length} deps)`);
    
    for (const [depName, version] of Object.entries(dependencies)) {
      if (!BUILTIN_MODULES.has(depName) && !ELECTRON_MODULES.has(depName)) {
        allDeps.add(depName);
        
        // Recursively get sub-dependencies
        const subDeps = getAllDependenciesRecursive(depName, nodeModulesPath, new Set(visited), depth + 1);
        for (const subDep of subDeps) {
          allDeps.add(subDep);
        }
      }
    }
  } catch (error) {
    console.log(`${'  '.repeat(depth)}❌ Error reading ${moduleName}: ${error.message}`);
  }
  
  return allDeps;
}

function findAllRequiredModules() {
  const nodeModulesPath = path.join(__dirname, '..', 'node_modules');
  
  console.log('🔍 Analyzing complete dependency tree for form-data and axios...');
  console.log('================================================================');
  
  // Get all dependencies for both axios and form-data
  const axiosDeps = getAllDependenciesRecursive('axios', nodeModulesPath);
  const formDataDeps = getAllDependenciesRecursive('form-data', nodeModulesPath);
  
  // Combine all dependencies
  const allDeps = new Set([...axiosDeps, ...formDataDeps, 'axios', 'form-data']);
  
  console.log('\n📊 Complete dependency analysis:');
  console.log('================================');
  console.log(`Total unique dependencies found: ${allDeps.size}`);
  
  return Array.from(allDeps).sort();
}

function checkAndInstallMissing(dependencies) {
  const nodeModulesPath = path.join(__dirname, '..', 'node_modules');
  const missing = [];
  const existing = [];
  
  console.log('\n🔍 Checking which dependencies are missing...');
  console.log('==============================================');
  
  for (const dep of dependencies) {
    const depPath = path.join(nodeModulesPath, dep);
    if (fs.existsSync(depPath)) {
      existing.push(dep);
      console.log(`✅ ${dep}`);
    } else {
      missing.push(dep);
      console.log(`❌ ${dep} - MISSING`);
    }
  }
  
  console.log(`\n📊 Status: ${existing.length} existing, ${missing.length} missing`);
  
  if (missing.length > 0) {
    console.log('\n📦 Installing missing dependencies...');
    console.log('====================================');
    
    const successful = [];
    const failed = [];
    
    for (const dep of missing) {
      try {
        console.log(`Installing ${dep}...`);
        execSync(`pnpm add ${dep}`, { stdio: 'pipe' });
        successful.push(dep);
        console.log(`✅ ${dep} installed successfully`);
      } catch (error) {
        failed.push(dep);
        console.log(`❌ ${dep} failed: ${error.message.split('\n')[0]}`);
      }
    }
    
    console.log(`\n📊 Installation results: ${successful.length} successful, ${failed.length} failed`);
    return { successful, failed, existing };
  }
  
  return { successful: [], failed: [], existing };
}

function updateAsarUnpack(allDependencies) {
  const packageJsonPath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
  
  // Create comprehensive asarUnpack list
  const asarUnpack = allDependencies.map(dep => `node_modules/${dep}/**/*`);
  
  packageJson.build.asarUnpack = asarUnpack;
  
  fs.writeFileSync(packageJsonPath, JSON.stringify(packageJson, null, 2));
  console.log(`\n🔧 Updated asarUnpack with ${asarUnpack.length} modules`);
}

function main() {
  console.log('🚀 Comprehensive Dependency Analyzer');
  console.log('====================================');
  
  try {
    // Find all required modules
    const allDependencies = findAllRequiredModules();
    
    // Check and install missing ones
    const { successful, failed, existing } = checkAndInstallMissing(allDependencies);
    
    // Update asarUnpack configuration
    const availableDeps = [...existing, ...successful];
    updateAsarUnpack(availableDeps);
    
    console.log('\n🎯 Final Summary');
    console.log('================');
    console.log(`Total dependencies analyzed: ${allDependencies.length}`);
    console.log(`Already installed: ${existing.length}`);
    console.log(`Successfully installed: ${successful.length}`);
    console.log(`Failed to install: ${failed.length}`);
    console.log(`Available for packaging: ${availableDeps.length}`);
    
    if (failed.length > 0) {
      console.log('\n❌ Failed dependencies:');
      failed.forEach(dep => console.log(`   - ${dep}`));
    }
    
    console.log('\n✅ Analysis complete! Ready to build and test.');
    
  } catch (error) {
    console.error('❌ Error during analysis:', error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { findAllRequiredModules, checkAndInstallMissing, updateAsarUnpack };