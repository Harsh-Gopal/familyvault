const fs = require('fs');
const path = require('path');

const electronDistDir = path.join(__dirname, '../dist/electron');

if (!fs.existsSync(electronDistDir)) {
  console.log('Electron dist directory does not exist');
  process.exit(0);
}

// Get all JS files in the electron dist directory
const files = fs.readdirSync(electronDistDir);
const jsFiles = files.filter(file => file.endsWith('.js'));

// First, update require statements in all JS files to use .cjs extensions
jsFiles.forEach(file => {
  const filePath = path.join(electronDistDir, file);
  let content = fs.readFileSync(filePath, 'utf8');
  
  // Update require statements to use .cjs extensions
  content = content.replace(/require\("\.\/([^"]+)\.js"\)/g, 'require("./$1.cjs")');
  content = content.replace(/require\('\.\/([^']+)\.js'\)/g, "require('./$1.cjs')");
  // Also handle require statements without .js extension (they need .cjs)
  content = content.replace(/require\("\.\/([^"]+)"\)/g, (match, filename) => {
    if (!filename.includes('.')) {
      return `require("./${filename}.cjs")`;
    }
    return match;
  });
  content = content.replace(/require\('\.\/([^']+)'\)/g, (match, filename) => {
    if (!filename.includes('.')) {
      return `require('./${filename}.cjs')`;
    }
    return match;
  });
  
  fs.writeFileSync(filePath, content);
});

// Then rename all JS files to .cjs
jsFiles.forEach(file => {
  const oldPath = path.join(electronDistDir, file);
  const newPath = path.join(electronDistDir, file.replace('.js', '.cjs'));
  fs.renameSync(oldPath, newPath);
  console.log(`Renamed ${file} to ${file.replace('.js', '.cjs')}`);
});

console.log('Successfully converted all Electron JS files to CJS format');