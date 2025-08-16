const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Complete list of all form-data dependencies discovered through analysis
const FORM_DATA_DEPENDENCIES = [
  'asynckit',
  'combined-stream',
  'delayed-stream',
  'es-errors',
  'es-set-tostringtag',
  'function-bind',
  'get-intrinsic',
  'has-tostringtag',
  'hasown',
  'mime-types',
  'mime-db',
  // Additional deep dependencies that may be missing
  'es-object-atoms',
  'es-define-property',
  'get-proto',
  'gopd',
  'has-symbols',
  'math-intrinsics',
  'call-bind-apply-helpers',
  'has-property-descriptors',
  'define-properties',
  'object-keys',
  'call-bind',
  'set-function-length',
  'define-data-property',
  'es-abstract'
];

function installDependency(dep) {
  try {
    console.log(`📦 Installing ${dep}...`);
    execSync(`pnpm add ${dep}`, { stdio: 'pipe' });
    console.log(`✅ Successfully installed ${dep}`);
    return true;
  } catch (error) {
    console.log(`❌ Failed to install ${dep}: ${error.message}`);
    return false;
  }
}

function updateAsarUnpack(dependencies) {
  const packageJsonPath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));
  
  // Create asarUnpack array with form-data and all its dependencies
  const asarUnpack = [
    'node_modules/form-data/**/*',
    ...dependencies.map(dep => `node_modules/${dep}/**/*`)
  ];
  
  packageJson.build.asarUnpack = asarUnpack;
  
  fs.writeFileSync(packageJsonPath, JSON.stringify(packageJson, null, 2));
  console.log(`✅ Updated asarUnpack configuration with ${asarUnpack.length} modules`);
}

function main() {
  console.log('🚀 Installing All Form-Data Dependencies');
  console.log('========================================');
  console.log(`Total dependencies to install: ${FORM_DATA_DEPENDENCIES.length}`);
  console.log('');
  
  const successful = [];
  const failed = [];
  
  for (const dep of FORM_DATA_DEPENDENCIES) {
    if (installDependency(dep)) {
      successful.push(dep);
    } else {
      failed.push(dep);
    }
  }
  
  console.log('');
  console.log('📊 Installation Summary');
  console.log('======================');
  console.log(`✅ Successfully installed: ${successful.length}`);
  console.log(`❌ Failed to install: ${failed.length}`);
  
  if (successful.length > 0) {
    console.log('');
    console.log('✅ Successfully installed:');
    successful.forEach(dep => console.log(`   - ${dep}`));
  }
  
  if (failed.length > 0) {
    console.log('');
    console.log('❌ Failed to install:');
    failed.forEach(dep => console.log(`   - ${dep}`));
  }
  
  // Update asarUnpack configuration
  console.log('');
  console.log('🔧 Updating asarUnpack configuration...');
  updateAsarUnpack(successful);
  
  console.log('');
  if (failed.length === 0) {
    console.log('🎉 All dependencies installed successfully!');
    console.log('   Ready to build and package the app.');
  } else {
    console.log('⚠️  Some dependencies failed to install.');
    console.log('   The app may still work if these are optional dependencies.');
  }
}

if (require.main === module) {
  main();
}

module.exports = { FORM_DATA_DEPENDENCIES, installDependency, updateAsarUnpack };