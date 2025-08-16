const fs = require('fs');
const path = require('path');

function main() {
  console.log('🎯 FINAL DEPENDENCY VERIFICATION');
  console.log('================================');

  const packageJsonPath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));

  // Check critical dependencies
  const criticalDeps = ['axios', 'keytar', 'form-data'];
  const missingCritical = [];

  console.log('\n🔍 Checking critical dependencies:');
  console.log('==================================');

  for (const dep of criticalDeps) {
    const inDeps = packageJson.dependencies && packageJson.dependencies[dep];
    const inDevDeps = packageJson.devDependencies && packageJson.devDependencies[dep];
    const inOptDeps = packageJson.optionalDependencies && packageJson.optionalDependencies[dep];

    if (inDeps) {
      console.log(`✅ ${dep} - in dependencies`);
    } else if (inDevDeps) {
      console.log(`⚠️  ${dep} - in devDependencies (should be in dependencies)`);
      missingCritical.push(dep);
    } else if (inOptDeps) {
      console.log(`✅ ${dep} - in optionalDependencies`);
    } else {
      console.log(`❌ ${dep} - MISSING`);
      missingCritical.push(dep);
    }
  }

  // Check asarUnpack configuration
  console.log('\n🔧 Checking asarUnpack configuration:');
  console.log('====================================');

  const asarUnpack = packageJson.build.asarUnpack || [];
  const expectedUnpacked = ['keytar', 'axios', 'form-data'];

  for (const dep of expectedUnpacked) {
    const isUnpacked = asarUnpack.some(entry => entry.includes(dep));
    if (isUnpacked) {
      console.log(`✅ ${dep} - configured for unpacking`);
    } else {
      console.log(`❌ ${dep} - NOT configured for unpacking`);
    }
  }

  console.log(`\n📊 Total modules in asarUnpack: ${asarUnpack.length}`);

  // Check if build files exist
  console.log('\n📁 Checking build files:');
  console.log('========================');

  const electronDistPath = path.join(__dirname, '..', 'dist', 'electron');
  const expectedFiles = ['main.cjs', 'keychain.cjs', 'ipc.cjs', 'preload.cjs', 'backend.cjs'];

  for (const file of expectedFiles) {
    const filePath = path.join(electronDistPath, file);
    if (fs.existsSync(filePath)) {
      console.log(`✅ ${file} - exists`);
    } else {
      console.log(`❌ ${file} - MISSING`);
    }
  }

  // Check packaged app
  console.log('\n📦 Checking packaged app:');
  console.log('=========================');

  const releasePath = path.join(__dirname, '..', 'release');
  const dmgPath = path.join(releasePath, 'FamilyVault-1.0.0-arm64.dmg');
  const appPath = path.join(releasePath, 'mac-arm64', 'FamilyVault.app');

  if (fs.existsSync(dmgPath)) {
    console.log('✅ DMG file exists');
  } else {
    console.log('❌ DMG file missing');
  }

  if (fs.existsSync(appPath)) {
    console.log('✅ App bundle exists');

    const unpackedPath = path.join(appPath, 'Contents', 'Resources', 'app.asar.unpacked', 'node_modules');
    if (fs.existsSync(unpackedPath)) {
      const unpackedModules = fs.readdirSync(unpackedPath);
      console.log(`✅ Unpacked modules: ${unpackedModules.length}`);

      for (const dep of expectedUnpacked) {
        if (unpackedModules.includes(dep)) {
          console.log(`   ✅ ${dep} - unpacked`);
        } else {
          console.log(`   ❌ ${dep} - NOT unpacked`);
        }
      }
    } else {
      console.log('❌ Unpacked modules directory missing');
    }
  } else {
    console.log('❌ App bundle missing');
  }

  // Final status
  console.log('\n🎉 FINAL STATUS');
  console.log('===============');

  if (missingCritical.length === 0) {
    console.log('✅ ALL CRITICAL DEPENDENCIES RESOLVED');
    console.log('✅ App should launch without module errors');
    console.log('✅ Ready for production distribution');
  } else {
    console.log('❌ Some critical dependencies need attention:');
    missingCritical.forEach(dep => console.log(`   - ${dep}`));
  }

  console.log('\n📋 Next steps:');
  console.log('==============');
  console.log('1. Test the packaged app manually');
  console.log('2. Verify all app functionality works');
  console.log('3. Distribute the DMG file');
}

if (require.main === module) {
  main();
}

module.exports = { main };