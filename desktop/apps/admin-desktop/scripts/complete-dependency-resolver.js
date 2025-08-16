const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Built-in Node.js modules that don't need to be installed
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

function extractAllRequires(content, filePath) {
  const requires = new Set();
  
  // Match require('module') and require("module")
  const requireRegex = /require\s*\(\s*['"`]([^'"`]+)['"`]\s*\)/g;
  let match;
  
  while ((match = requireRegex.exec(content)) !== null) {
    const moduleName = match[1];
    
    // Skip relative requires (they're internal to the app)
    if (moduleName.startsWith('./') || moduleName.startsWith('../')) {
      continue;
    }
    
    // Extract the base module name (before any subpath)
    const baseModule = moduleName.split('/')[0];
    requires.add(baseModule);
  }
  
  // Also check for dynamic imports
  const importRegex = /import\s*\(\s*['"`]([^'"`]+)['"`]\s*\)/g;
  while ((match = importRegex.exec(content)) !== null) {
    const moduleName = match[1];
    if (!moduleName.startsWith('./') && !moduleName.startsWith('../')) {
      const baseModule = moduleName.split('/')[0];
      requires.add(baseModule);
    }
  }
  
  return requires;
}

function scanAllElectronFiles(electronDistPath) {
  const allRequires = new Set();
  const fileRequires = new Map();
  
  if (!fs.existsSync(electronDistPath)) {
    console.error(`❌ Electron dist directory not found: ${electronDistPath}`);
    return { allRequires, fileRequires };
  }
  
  const files = fs.readdirSync(electronDistPath);
  const jsFiles = files.filter(file => file.endsWith('.cjs') || file.endsWith('.js'));
  
  console.log(`🔍 Scanning ${jsFiles.length} Electron files for ALL dependencies...`);
  
  for (const file of jsFiles) {
    const filePath = path.join(electronDistPath, file);
    const content = fs.readFileSync(filePath, 'utf8');
    const requires = extractAllRequires(content, filePath);
    
    fileRequires.set(file, requires);
    
    console.log(`   📄 ${file}: found ${requires.size} require statements`);
    for (const req of requires) {
      console.log(`      - ${req}`);
    }
    
    for (const moduleName of requires) {
      allRequires.add(moduleName);
    }
  }
  
  return { allRequires, fileRequires };
}

function getAllDependenciesRecursive(moduleName, nodeModulesPath, visited = new Set(), depth = 0) {
  const maxDepth = 8; // Prevent infinite recursion
  if (visited.has(moduleName) || depth > maxDepth) {
    return new Set();
  }
  
  visited.add(moduleName);
  const allDeps = new Set();
  
  const modulePath = path.join(nodeModulesPath, moduleName);
  const packageJsonPath = path.join(modulePath, 'package.json');
  
  if (!fs.existsSync(packageJsonPath)) {
    return allDeps;
  }
  
  try {
    const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
    const dependencies = {
      ...packageJson.dependencies,
      ...packageJson.peerDependencies,
      ...packageJson.optionalDependencies
    };
    
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
    // Ignore errors for optional dependencies
  }
  
  return allDeps;
}

function analyzeCurrentPackageJson() {
  const packageJsonPath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
  
  const allCurrentDeps = {
    ...packageJson.dependencies,
    ...packageJson.devDependencies
  };
  
  return { packageJson, allCurrentDeps };
}

function installMissingDependencies(missingDeps) {
  const successful = [];
  const failed = [];
  
  if (missingDeps.length === 0) {
    console.log('✅ No missing dependencies to install');
    return { successful, failed };
  }
  
  console.log(`\n📦 Installing ${missingDeps.length} missing dependencies...`);
  console.log('='.repeat(50));
  
  // Try to install all at once first
  try {
    const depsString = missingDeps.join(' ');
    console.log(`Installing: ${depsString}`);
    execSync(`pnpm add ${depsString}`, { stdio: 'pipe' });
    successful.push(...missingDeps);
    console.log(`✅ Successfully installed all ${missingDeps.length} dependencies`);
  } catch (error) {
    console.log('❌ Batch install failed, trying individually...');
    
    // If batch fails, try one by one
    for (const dep of missingDeps) {
      try {
        console.log(`Installing ${dep}...`);
        execSync(`pnpm add ${dep}`, { stdio: 'pipe' });
        successful.push(dep);
        console.log(`✅ ${dep}`);
      } catch (error) {
        failed.push(dep);
        console.log(`❌ ${dep}: ${error.message.split('\n')[0]}`);
      }
    }
  }
  
  return { successful, failed };
}

function updatePackageJsonAndAsarUnpack(allRequiredDeps) {
  const packageJsonPath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
  
  // Create comprehensive asarUnpack list
  const asarUnpack = allRequiredDeps
    .filter(dep => !BUILTIN_MODULES.has(dep) && !ELECTRON_MODULES.has(dep))
    .sort()
    .map(dep => `node_modules/${dep}/**/*`);
  
  packageJson.build.asarUnpack = asarUnpack;
  
  fs.writeFileSync(packageJsonPath, JSON.stringify(packageJson, null, 2));
  console.log(`\n🔧 Updated asarUnpack with ${asarUnpack.length} modules`);
  
  return asarUnpack.length;
}

function main() {
  console.log('🚀 COMPLETE DEPENDENCY RESOLVER');
  console.log('===============================');
  console.log('This will scan ALL files and resolve ALL dependencies once and for all!');
  console.log('');
  
  const projectRoot = path.join(__dirname, '..');
  const electronDistPath = path.join(projectRoot, 'dist', 'electron');
  const nodeModulesPath = path.join(projectRoot, 'node_modules');
  
  // Step 1: Scan all Electron files for required modules
  console.log('📋 STEP 1: Scanning Electron files for required modules');
  console.log('='.repeat(55));
  const { allRequires, fileRequires } = scanAllElectronFiles(electronDistPath);
  
  // Filter out built-in and Electron modules
  const externalRequires = Array.from(allRequires).filter(
    dep => !BUILTIN_MODULES.has(dep) && !ELECTRON_MODULES.has(dep)
  );
  
  console.log(`\n📊 Found ${externalRequires.length} external module requirements:`);
  externalRequires.sort().forEach(dep => console.log(`   - ${dep}`));
  
  // Step 2: Get all recursive dependencies
  console.log(`\n📋 STEP 2: Analyzing recursive dependencies`);
  console.log('='.repeat(42));
  
  const allDependencies = new Set(externalRequires);
  
  for (const dep of externalRequires) {
    console.log(`Analyzing ${dep}...`);
    const subDeps = getAllDependenciesRecursive(dep, nodeModulesPath);
    for (const subDep of subDeps) {
      allDependencies.add(subDep);
    }
  }
  
  const allRequiredDeps = Array.from(allDependencies).sort();
  console.log(`\n📊 Total dependencies required: ${allRequiredDeps.length}`);
  
  // Step 3: Check what's missing
  console.log(`\n📋 STEP 3: Checking current installation status`);
  console.log('='.repeat(44));
  
  const { packageJson, allCurrentDeps } = analyzeCurrentPackageJson();
  
  const missing = [];
  const existing = [];
  const inDevDeps = [];
  
  for (const dep of allRequiredDeps) {
    if (allCurrentDeps[dep]) {
      existing.push(dep);
      if (packageJson.devDependencies && packageJson.devDependencies[dep]) {
        inDevDeps.push(dep);
      }
    } else {
      missing.push(dep);
    }
  }
  
  console.log(`✅ Already installed: ${existing.length}`);
  console.log(`❌ Missing: ${missing.length}`);
  console.log(`⚠️  In devDependencies (should be in dependencies): ${inDevDeps.length}`);
  
  if (inDevDeps.length > 0) {
    console.log('\n⚠️  Dependencies in devDependencies that should be in dependencies:');
    inDevDeps.forEach(dep => console.log(`   - ${dep}`));
  }
  
  if (missing.length > 0) {
    console.log('\n❌ Missing dependencies:');
    missing.forEach(dep => console.log(`   - ${dep}`));
  }
  
  // Step 4: Install missing dependencies
  console.log(`\n📋 STEP 4: Installing missing dependencies`);
  console.log('='.repeat(40));
  
  const { successful, failed } = installMissingDependencies(missing);
  
  // Step 5: Move devDependencies to dependencies if needed
  if (inDevDeps.length > 0) {
    console.log(`\n📋 STEP 5: Moving runtime dependencies from devDependencies`);
    console.log('='.repeat(58));
    
    for (const dep of inDevDeps) {
      const version = packageJson.devDependencies[dep];
      packageJson.dependencies[dep] = version;
      delete packageJson.devDependencies[dep];
      console.log(`✅ Moved ${dep} to dependencies`);
    }
    
    fs.writeFileSync(path.join(__dirname, '..', 'package.json'), JSON.stringify(packageJson, null, 2));
  }
  
  // Step 6: Update asarUnpack configuration
  console.log(`\n📋 STEP 6: Updating asarUnpack configuration`);
  console.log('='.repeat(42));
  
  const finalDeps = [...existing, ...successful];
  const asarCount = updatePackageJsonAndAsarUnpack(finalDeps);
  
  // Final summary
  console.log(`\n🎯 FINAL SUMMARY`);
  console.log('='.repeat(16));
  console.log(`📊 Total dependencies analyzed: ${allRequiredDeps.length}`);
  console.log(`✅ Successfully resolved: ${finalDeps.length}`);
  console.log(`❌ Failed to install: ${failed.length}`);
  console.log(`🔧 ASAR unpacked modules: ${asarCount}`);
  console.log(`📦 Moved from devDependencies: ${inDevDeps.length}`);
  
  if (failed.length > 0) {
    console.log('\n❌ Failed to install:');
    failed.forEach(dep => console.log(`   - ${dep}`));
    console.log('\n⚠️  You may need to install these manually or they may be optional');
  }
  
  console.log('\n🎉 DEPENDENCY RESOLUTION COMPLETE!');
  console.log('==================================');
  console.log('✅ All required dependencies should now be properly configured');
  console.log('✅ Ready to build and package the application');
  console.log('✅ Run: pnpm electron:build:safe');
}

if (require.main === module) {
  main();
}

module.exports = { main, scanAllElectronFiles, getAllDependenciesRecursive };