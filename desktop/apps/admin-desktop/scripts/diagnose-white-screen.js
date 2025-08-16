const fs = require('fs');
const path = require('path');

function main() {
  console.log('🔍 WHITE SCREEN DIAGNOSTIC');
  console.log('=========================');
  
  // Check build structure
  console.log('\n📁 Checking build structure:');
  console.log('============================');
  
  const distPath = path.join(__dirname, '..', 'dist');
  const electronDistPath = path.join(distPath, 'electron');
  
  // Check if dist directory exists
  if (fs.existsSync(distPath)) {
    console.log('✅ dist/ directory exists');
    
    const distFiles = fs.readdirSync(distPath);
    console.log(`   Files: ${distFiles.join(', ')}`);
    
    // Check for index.html
    const indexPath = path.join(distPath, 'index.html');
    if (fs.existsSync(indexPath)) {
      console.log('✅ dist/index.html exists');
      
      // Check index.html content
      const indexContent = fs.readFileSync(indexPath, 'utf8');
      
      // Check for problematic paths
      if (indexContent.includes('href="/')) {
        console.log('⚠️  Found absolute paths in index.html (should be relative)');
        const absolutePaths = indexContent.match(/(?:href|src)="\/[^"]+"/g);
        if (absolutePaths) {
          absolutePaths.forEach(path => console.log(`   - ${path}`));
        }
      }
      
      // Check for assets
      if (indexContent.includes('./assets/')) {
        console.log('✅ Assets use relative paths');
      } else {
        console.log('❌ Assets may not use relative paths');
      }
      
    } else {
      console.log('❌ dist/index.html missing');
    }
    
    // Check assets directory
    const assetsPath = path.join(distPath, 'assets');
    if (fs.existsSync(assetsPath)) {
      console.log('✅ dist/assets/ directory exists');
      const assetFiles = fs.readdirSync(assetsPath);
      console.log(`   Asset files: ${assetFiles.length}`);
      assetFiles.forEach(file => console.log(`   - ${file}`));
    } else {
      console.log('❌ dist/assets/ directory missing');
    }
    
  } else {
    console.log('❌ dist/ directory missing - run pnpm build first');
    return;
  }
  
  // Check electron files
  console.log('\n⚡ Checking Electron files:');
  console.log('===========================');
  
  if (fs.existsSync(electronDistPath)) {
    console.log('✅ dist/electron/ directory exists');
    
    const mainPath = path.join(electronDistPath, 'main.cjs');
    if (fs.existsSync(mainPath)) {
      console.log('✅ dist/electron/main.cjs exists');
      
      // Check main.cjs for loadFile path
      const mainContent = fs.readFileSync(mainPath, 'utf8');
      
      if (mainContent.includes('loadFile')) {
        const loadFileMatch = mainContent.match(/loadFile\([^)]+\)/);
        if (loadFileMatch) {
          console.log(`   loadFile call: ${loadFileMatch[0]}`);
        }
      }
      
      // Check for isDev detection
      if (mainContent.includes('NODE_ENV')) {
        console.log('✅ Development mode detection present');
      } else {
        console.log('⚠️  No development mode detection found');
      }
      
    } else {
      console.log('❌ dist/electron/main.cjs missing');
    }
  } else {
    console.log('❌ dist/electron/ directory missing');
  }
  
  // Check packaged app structure
  console.log('\n📦 Checking packaged app:');
  console.log('=========================');
  
  const releasePath = path.join(__dirname, '..', 'release');
  const appPath = path.join(releasePath, 'mac-arm64', 'FamilyVault.app');
  
  if (fs.existsSync(appPath)) {
    console.log('✅ Packaged app exists');
    
    const resourcesPath = path.join(appPath, 'Contents', 'Resources');
    const appAsarPath = path.join(resourcesPath, 'app.asar');
    
    if (fs.existsSync(appAsarPath)) {
      console.log('✅ app.asar exists');
    } else {
      console.log('❌ app.asar missing');
    }
    
    // Check if frontend files are in the right place
    // Note: We can't easily inspect inside asar, but we can check the build config
    
  } else {
    console.log('❌ Packaged app missing - run pnpm electron:build first');
  }
  
  // Check Vite config
  console.log('\n⚙️  Checking Vite configuration:');
  console.log('================================');
  
  const viteConfigPath = path.join(__dirname, '..', 'vite.config.ts');
  if (fs.existsSync(viteConfigPath)) {
    console.log('✅ vite.config.ts exists');
    
    const viteConfig = fs.readFileSync(viteConfigPath, 'utf8');
    
    if (viteConfig.includes('base:')) {
      const baseMatch = viteConfig.match(/base:\s*['"`]([^'"`]+)['"`]/);
      if (baseMatch) {
        console.log(`   Base path: ${baseMatch[1]}`);
      }
    } else {
      console.log('   No explicit base path set');
    }
    
  } else {
    console.log('❌ vite.config.ts missing');
  }
  
  console.log('\n💡 RECOMMENDATIONS:');
  console.log('===================');
  console.log('1. Enable DevTools temporarily in main.ts to see console errors');
  console.log('2. Check that all asset paths in index.html are relative (./assets/)');
  console.log('3. Ensure Vite base is set to "./" for Electron');
  console.log('4. Verify the loadFile path in main.ts points to the correct location');
  console.log('5. Check browser console in the packaged app for JavaScript errors');
}

if (require.main === module) {
  main();
}

module.exports = { main };