const fs = require('fs');
const path = require('path');

// Built-in Node.js modules that don't need to be in node_modules
const BUILTIN_MODULES = new Set([
  'assert', 'buffer', 'child_process', 'cluster', 'crypto', 'dgram', 'dns',
  'domain', 'events', 'fs', 'http', 'https', 'net', 'os', 'path', 'punycode',
  'querystring', 'readline', 'stream', 'string_decoder', 'timers', 'tls',
  'tty', 'url', 'util', 'v8', 'vm', 'zlib', 'constants', 'module',
  'process', 'console', 'inspector', 'perf_hooks', 'trace_events',
  'worker_threads', 'async_hooks', 'http2', 'repl', 'wasi'
]);

// Electron modules that are provided by Electron
const ELECTRON_MODULES = new Set([
  'electron', 'electron/main', 'electron/renderer', 'electron/common'
]);

function extractRequires(content) {
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
  
  return requires;
}

function checkModuleExists(moduleName, nodeModulesPath) {
  const modulePath = path.join(nodeModulesPath, moduleName);
  return fs.existsSync(modulePath);
}

function scanElectronFiles(electronDistPath, nodeModulesPath) {
  const missingModules = new Set();
  const allRequires = new Set();
  
  if (!fs.existsSync(electronDistPath)) {
    console.error(`❌ Electron dist directory not found: ${electronDistPath}`);
    process.exit(1);
  }
  
  const files = fs.readdirSync(electronDistPath);
  const jsFiles = files.filter(file => file.endsWith('.cjs') || file.endsWith('.js'));
  
  console.log(`🔍 Scanning ${jsFiles.length} Electron files for dependencies...`);
  
  for (const file of jsFiles) {
    const filePath = path.join(electronDistPath, file);
    const content = fs.readFileSync(filePath, 'utf8');
    const requires = extractRequires(content);
    
    console.log(`   📄 ${file}: found ${requires.size} require statements`);
    
    for (const moduleName of requires) {
      allRequires.add(moduleName);
      
      // Skip built-in and Electron modules
      if (BUILTIN_MODULES.has(moduleName) || ELECTRON_MODULES.has(moduleName)) {
        continue;
      }
      
      // Check if module exists in node_modules
      if (!checkModuleExists(moduleName, nodeModulesPath)) {
        missingModules.add(moduleName);
        console.log(`   ❌ Missing: ${moduleName} (required by ${file})`);
      } else {
        console.log(`   ✅ Found: ${moduleName}`);
      }
    }
  }
  
  return { missingModules, allRequires };
}

function main() {
  const projectRoot = path.join(__dirname, '..');
  const electronDistPath = path.join(projectRoot, 'dist', 'electron');
  const nodeModulesPath = path.join(projectRoot, 'node_modules');
  
  console.log('🚀 Runtime Dependency Checker');
  console.log('==============================');
  console.log(`Project root: ${projectRoot}`);
  console.log(`Electron dist: ${electronDistPath}`);
  console.log(`Node modules: ${nodeModulesPath}`);
  console.log('');
  
  const { missingModules, allRequires } = scanElectronFiles(electronDistPath, nodeModulesPath);
  
  console.log('');
  console.log('📊 Summary');
  console.log('==========');
  console.log(`Total unique requires found: ${allRequires.size}`);
  console.log(`Built-in modules: ${Array.from(allRequires).filter(m => BUILTIN_MODULES.has(m)).length}`);
  console.log(`Electron modules: ${Array.from(allRequires).filter(m => ELECTRON_MODULES.has(m)).length}`);
  console.log(`External modules: ${Array.from(allRequires).filter(m => !BUILTIN_MODULES.has(m) && !ELECTRON_MODULES.has(m)).length}`);
  console.log(`Missing modules: ${missingModules.size}`);
  
  if (missingModules.size > 0) {
    console.log('');
    console.log('❌ MISSING DEPENDENCIES:');
    console.log('========================');
    for (const moduleName of Array.from(missingModules).sort()) {
      console.log(`   - ${moduleName}`);
    }
    console.log('');
    console.log('💡 To fix these issues:');
    console.log('   1. Add missing modules to package.json dependencies');
    console.log('   2. Run: pnpm install');
    console.log('   3. Add modules to asarUnpack in electron-builder config if needed');
    console.log('');
    process.exit(1);
  } else {
    console.log('');
    console.log('✅ All dependencies are available!');
    console.log('   Ready for packaging.');
    process.exit(0);
  }
}

if (require.main === module) {
  main();
}

module.exports = { extractRequires, checkModuleExists, scanElectronFiles };