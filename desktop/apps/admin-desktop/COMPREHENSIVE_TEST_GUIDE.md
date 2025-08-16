# 🧪 Comprehensive External Drive Testing Guide

## 🎯 Test Objectives

Test all FamilyVault features with your **SanDisk 128GB** external drive:
1. ✅ Device detection and selection
2. ✅ File upload (backup) functionality  
3. ✅ File download (restore) functionality
4. ✅ Multi-user storage with quotas
5. ✅ Storage limits and quota enforcement
6. ✅ Concurrent user operations

## 🚀 Pre-Test Setup

### 1. Launch the Application
```bash
cd desktop/apps/admin-desktop
./run-app.sh built
```

### 2. Verify External Drive Detection
- Your **SanDisk 128** should appear in the Vault device list
- Should show ~114GB capacity with usage bar
- Should have "external" (blue) and "removable" (orange) badges

## 📋 Test Scenarios

### **Test 1: Basic Vault Setup & File Backup**

#### Steps:
1. Navigate to "Vault" tab
2. Select "SanDisk 128" from device list
3. Verify vault creation at `/Volumes/Sandisk 128/FamilyVault/<groupId>/<userId>/`
4. Upload test files (photos, documents, videos)
5. Watch progress bars during upload
6. Verify files appear in the file list

#### Expected Results:
- ✅ Green "Vault Configured" status
- ✅ Directory auto-created on external drive
- ✅ Files uploaded with progress tracking
- ✅ File list shows uploaded files with sizes and dates
- ✅ Storage usage updates correctly

#### Test Files to Use:
- Small files: Text documents (< 1MB)
- Medium files: Photos (1-10MB)
- Large files: Videos (50-100MB)

---

### **Test 2: File Download (Restore)**

#### Steps:
1. From the uploaded file list, click "Download" on various files
2. Choose different save locations
3. Verify downloaded files match originals
4. Test downloading multiple files

#### Expected Results:
- ✅ Save dialog opens with original filename
- ✅ Files download to chosen locations
- ✅ Downloaded files are identical to originals
- ✅ Download progress shows for large files

---

### **Test 3: Multi-User Storage Setup**

#### Steps:
1. **User 1 Setup:**
   - Create/join a group as User 1
   - Set up vault on SanDisk 128
   - Upload files (use ~2GB of test data)
   - Note the directory: `/Volumes/Sandisk 128/FamilyVault/<groupId>/<user1Id>/`

2. **User 2 Setup:**
   - Create a second user account (or simulate)
   - Join the same group as User 2
   - Set up vault on the same SanDisk 128
   - Upload different files (use ~2GB of test data)
   - Note the directory: `/Volumes/Sandisk 128/FamilyVault/<groupId>/<user2Id>/`

#### Expected Results:
- ✅ Separate directories created for each user
- ✅ Each user sees only their own files
- ✅ Storage usage tracked separately
- ✅ No file conflicts between users

---

### **Test 4: Storage Quotas & Limits**

#### Steps:
1. **Set Quotas:**
   - User 1: Set 5GB quota
   - User 2: Set 3GB quota

2. **Test Quota Enforcement:**
   - Try uploading files that exceed quota
   - Verify rejection with clear error message
   - Upload files within quota limits

3. **Monitor Usage:**
   - Watch storage bars update in real-time
   - Verify accurate usage calculations
   - Check remaining space calculations

#### Expected Results:
- ✅ Quota enforcement prevents oversized uploads
- ✅ Clear error messages for quota violations
- ✅ Accurate storage usage tracking
- ✅ Real-time usage bar updates

---

### **Test 5: Concurrent Operations**

#### Steps:
1. **Simultaneous Uploads:**
   - User 1: Start uploading large files
   - User 2: Start uploading different files at same time
   - Monitor both progress bars

2. **Mixed Operations:**
   - User 1: Upload files
   - User 2: Download files simultaneously
   - Verify no conflicts or corruption

#### Expected Results:
- ✅ Both users can upload simultaneously
- ✅ No file corruption or conflicts
- ✅ Progress tracking works for both users
- ✅ Performance remains acceptable

---

### **Test 6: Physical Drive Verification**

#### Steps:
1. **Finder Verification:**
   - Click "Open in Finder" for each user
   - Navigate to `/Volumes/Sandisk 128/FamilyVault/`
   - Verify directory structure:
     ```
     /Volumes/Sandisk 128/FamilyVault/
     ├── <groupId>/
     │   ├── <user1Id>/
     │   │   ├── .vault.manifest.json
     │   │   ├── <file1-uuid>
     │   │   └── <file2-uuid>
     │   └── <user2Id>/
     │       ├── .vault.manifest.json
     │       ├── <file3-uuid>
     │       └── <file4-uuid>
     ```

2. **Manifest Verification:**
   - Open `.vault.manifest.json` files
   - Verify file metadata is correct
   - Check SHA-256 hashes are present

#### Expected Results:
- ✅ Proper directory structure on external drive
- ✅ Files stored with UUID names for security
- ✅ Manifest files contain accurate metadata
- ✅ Files are physically on the external drive

---

### **Test 7: Drive Portability**

#### Steps:
1. **Unplug Test:**
   - Safely eject the SanDisk 128 drive
   - Verify app handles disconnection gracefully
   - Reconnect drive and verify detection

2. **Cross-Device Test:**
   - Take drive to another Mac (if available)
   - Install FamilyVault on second Mac
   - Verify files are accessible and downloadable

#### Expected Results:
- ✅ Graceful handling of drive disconnection
- ✅ Automatic re-detection when reconnected
- ✅ Files remain accessible on other devices
- ✅ True portability of vault data

---

## 📊 Performance Benchmarks

### Upload Performance:
- **Small files (< 1MB)**: Should complete in < 1 second
- **Medium files (1-10MB)**: Should show progress, complete in < 10 seconds
- **Large files (50-100MB)**: Should show detailed progress, complete in < 60 seconds

### Download Performance:
- Similar to upload times
- Progress indication for files > 10MB

### Concurrent Operations:
- 2 users uploading simultaneously should maintain > 50% of single-user speed
- No timeouts or failures during concurrent operations

## 🐛 Troubleshooting

### Drive Not Detected:
1. Check if drive appears in Finder
2. Try unplugging and reconnecting
3. Use "Choose folder..." as fallback
4. Check drive format (should be FAT32, exFAT, or APFS)

### Upload Failures:
1. Check available space on drive
2. Verify drive is not write-protected
3. Check file permissions
4. Try smaller files first

### Performance Issues:
1. USB 2.0 drives will be slower than USB 3.0
2. Large files take longer - this is normal
3. Concurrent operations may reduce individual speeds

## ✅ Success Criteria

### Must Pass:
- [ ] SanDisk 128 detected with correct capacity
- [ ] Files upload successfully with progress
- [ ] Files download successfully 
- [ ] Multi-user directories created correctly
- [ ] Quota enforcement works
- [ ] Files physically stored on external drive
- [ ] No data corruption or loss

### Performance Targets:
- [ ] Upload speeds > 10MB/s for large files
- [ ] UI remains responsive during operations
- [ ] Concurrent operations work without conflicts
- [ ] Memory usage stays reasonable (< 500MB)

## 🎬 Demo Preparation

After successful testing, you'll have:
1. **Real external drive** with actual user data
2. **Multi-user setup** showing directory separation
3. **Upload/download workflow** demonstrating backup/restore
4. **Quota management** showing storage limits
5. **Portable storage** that works across devices

Perfect for demonstrating the practical value of FamilyVault!

## 📝 Test Results Log

Use this section to record your test results:

```
Test 1 - Basic Setup: ✅ PASS / ❌ FAIL
Notes: ________________________________

Test 2 - Download: ✅ PASS / ❌ FAIL  
Notes: ________________________________

Test 3 - Multi-User: ✅ PASS / ❌ FAIL
Notes: ________________________________

Test 4 - Quotas: ✅ PASS / ❌ FAIL
Notes: ________________________________

Test 5 - Concurrent: ✅ PASS / ❌ FAIL
Notes: ________________________________

Test 6 - Physical: ✅ PASS / ❌ FAIL
Notes: ________________________________

Test 7 - Portability: ✅ PASS / ❌ FAIL
Notes: ________________________________
```