const fs = require('fs');
const path = require('path');

function getPackageDependencies(packagePath) {
  try {
    const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
    return packageJson.dependencies || {};
  } catch (error) {
    console.warn(`Could not read package.json at ${packagePath}:`, error.message);
    return {};
  }
}

function getAllDependencies(moduleName, nodeModulesPath, visited = new Set()) {
  if (visited.has(moduleName)) {
    return new Set(); // Avoid circular dependencies
  }
  visited.add(moduleName);
  
  const allDeps = new Set();
  const packagePath = path.join(nodeModulesPath, moduleName, 'package.json');
  
  if (!fs.existsSync(packagePath)) {
    console.warn(`Package not found: ${moduleName}`);
    return allDeps;
  }
  
  const dependencies = getPackageDependencies(packagePath);
  
  for (const depName of Object.keys(dependencies)) {
    allDeps.add(depName);
    
    // Recursively get dependencies of dependencies
    const subDeps = getAllDependencies(depName, nodeModulesPath, new Set(visited));
    for (const subDep of subDeps) {
      allDeps.add(subDep);
    }
  }
  
  return allDeps;
}

function main() {
  const nodeModulesPath = path.join(__dirname, '..', 'node_modules');
  
  console.log('🔍 Analyzing form-data dependency tree...');
  console.log('==========================================');
  
  const formDataDeps = getAllDependencies('form-data', nodeModulesPath);
  
  console.log('\n📦 Complete form-data dependency tree:');
  console.log('======================================');
  
  const sortedDeps = Array.from(formDataDeps).sort();
  for (const dep of sortedDeps) {
    const depPath = path.join(nodeModulesPath, dep);
    const exists = fs.existsSync(depPath);
    console.log(`${exists ? '✅' : '❌'} ${dep}`);
  }
  
  console.log('\n📋 Suggested asarUnpack configuration:');
  console.log('=====================================');
  console.log('"asarUnpack": [');
  console.log('  "node_modules/form-data/**/*",');
  for (const dep of sortedDeps) {
    console.log(`  "node_modules/${dep}/**/*",`);
  }
  console.log(']');
  
  console.log('\n📋 Suggested package.json dependencies:');
  console.log('=======================================');
  console.log('"dependencies": {');
  console.log('  "form-data": "^4.0.0",');
  for (const dep of sortedDeps) {
    const packagePath = path.join(nodeModulesPath, dep, 'package.json');
    if (fs.existsSync(packagePath)) {
      const pkg = getPackageDependencies(packagePath);
      const version = pkg.version || '^1.0.0';
      console.log(`  "${dep}": "^${version.replace(/^\^/, '')}",`);
    }
  }
  console.log('}');
  
  console.log(`\n📊 Total dependencies found: ${formDataDeps.size}`);
}

if (require.main === module) {
  main();
}

module.exports = { getAllDependencies, getPackageDependencies };