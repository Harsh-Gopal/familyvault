# External Drive Testing Guide

## 🔍 Your Setup Detected

Based on the system scan, your setup includes:
- **Internal Drive**: Macintosh HD (460GB, 32% used)
- **External Drive**: SanDisk 128 (114GB, ~0% used) ✅

## 🧪 Testing Steps

### 1. Launch the App
```bash
cd desktop/apps/admin-desktop
./run-app.sh built
```

### 2. Navigate to Vault
- Click on the "Vault" tab in the left sidebar
- You should see a device list showing:
  - **Macintosh HD** (Internal drive)
  - **SanDisk 128** (External drive) - This is your 128GB pen drive!
  - **Choose folder...** (Manual selection option)

### 3. Select Your External Drive
- Click on "SanDisk 128" 
- The app should show:
  - Device name: "SanDisk 128"
  - Type: "external" (with blue badge)
  - Capacity: ~114GB total
  - Usage bar showing current usage
  - "removable" badge (orange)

### 4. Test Vault Creation
- After selecting the drive, the app will automatically create:
  ```
  /Volumes/Sandisk 128/FamilyVault/<groupId>/<userId>/
  ```
- You should see a green "Vault Configured" card
- The "Open in Finder" button should open the created directory

### 5. Test File Upload
- Click "Upload Files" button
- Select some test files (photos, documents, etc.)
- Watch the progress bar during upload
- Files should be copied to your external drive
- Check the storage usage updates

### 6. Verify Physical Files
- Click "Open in Finder" 
- Navigate to `/Volumes/Sandisk 128/FamilyVault/`
- You should see the folder structure and uploaded files
- Check the `.vault.manifest.json` file for metadata

## 🎯 Expected Results

### Device Detection
- ✅ SanDisk 128 appears in device list
- ✅ Shows as "external" type with blue badge
- ✅ Shows as "removable" with orange badge
- ✅ Displays correct capacity (~114GB)
- ✅ Shows current usage percentage

### Vault Creation
- ✅ Creates directory structure automatically
- ✅ Path: `/Volumes/Sandisk 128/FamilyVault/<groupId>/<userId>/`
- ✅ Creates `.vault.manifest.json` file
- ✅ Shows green "Vault Configured" status

### File Operations
- ✅ File upload works with progress tracking
- ✅ Files are physically stored on external drive
- ✅ Manifest file is updated with file metadata
- ✅ Storage usage updates correctly
- ✅ "Open in Finder" opens correct location

## 🐛 Troubleshooting

### Drive Not Detected
- Ensure the drive is properly mounted
- Check if it appears in Finder sidebar
- Try unplugging and reconnecting the drive
- Use "Choose folder..." as fallback

### Permission Issues
- macOS may ask for permission to access external drives
- Grant permission when prompted
- Check System Preferences > Security & Privacy

### Upload Failures
- Ensure sufficient space on external drive
- Check drive is not write-protected
- Verify drive format is compatible (FAT32, exFAT, APFS)

## 📊 Performance Notes

### External Drive Performance
- USB 3.0/3.1 drives: Fast upload speeds
- USB 2.0 drives: Slower but functional
- File size affects upload time
- Progress bar shows real-time status

### Storage Efficiency
- Files stored with UUID names for security
- Original filenames preserved in manifest
- SHA-256 hashes for integrity verification
- Quota enforcement prevents overfilling

## 🎬 Demo Scenario

Perfect demo flow with your external drive:

1. **Show Device Detection** (10s)
   - "Here's real device detection - see your SanDisk 128 external drive!"
   - Point out the capacity bar and external/removable badges

2. **Select External Drive** (5s)
   - Click on SanDisk 128
   - "Auto-creates the vault structure on your external drive"

3. **Upload Files** (15s)
   - Upload some photos or documents
   - "Files are securely stored on your external drive with progress tracking"

4. **Show Physical Storage** (10s)
   - Click "Open in Finder"
   - "Files are actually on your external drive - you can take it anywhere!"

5. **Highlight Benefits** (10s)
   - "Portable storage - unplug and take your vault with you"
   - "Works with any external drive - USB, SSD, even network drives"

## 🚀 Next Steps

After successful testing:
1. Try with different file types and sizes
2. Test quota enforcement with large files
3. Try unplugging/reconnecting the drive
4. Test with multiple external drives
5. Verify data persistence across app restarts

Your 128GB SanDisk drive is perfect for demonstrating the real-world utility of the vault system!