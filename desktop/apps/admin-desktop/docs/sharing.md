# Sharing System Documentation

## Overview

The sharing system replaces the clipboard-only invitation sharing with a native macOS share sheet-style interface that detects installed applications and provides appropriate sharing methods for FamilyVault invitations.

## Architecture

### Main Process (sharing.ts)

The sharing functionality is implemented in the main Electron process to access system-level APIs for app detection and native sharing capabilities.

#### Key Components:

1. **App Detection**: Scans standard macOS application directories for known apps
2. **Share Target Management**: Maintains a registry of supported sharing applications
3. **Native Integration**: Uses macOS-specific APIs (AppleScript, URL schemes) for sharing
4. **Fallback Support**: Provides clipboard copying as a universal fallback

## Supported Applications

### Built-in macOS Apps

| App | Bundle ID | Location | Share Method |
|-----|-----------|----------|--------------|
| Mail | `com.apple.mail` | `/System/Applications/Mail.app` | `mailto:` URL scheme |
| Messages | `com.apple.iChat` | `/System/Applications/Messages.app` | AppleScript + SMS fallback |
| Notes | `com.apple.Notes` | `/System/Applications/Notes.app` | AppleScript |

### Third-Party Apps

| App | Bundle ID | Location | Share Method |
|-----|-----------|----------|--------------|
| WhatsApp | `WhatsApp` | `/Applications/WhatsApp.app` | `whatsapp://` URL scheme |
| Telegram | `ru.keepcoder.Telegram` | `/Applications/Telegram.app` | `tg://` URL scheme |
| Telegram Desktop | `org.telegram.desktop` | `/Applications/Telegram Desktop.app` | `tg://` URL scheme |

### Universal Fallback

| Method | Always Available | Implementation |
|--------|------------------|----------------|
| Copy to Clipboard | ✅ | Electron clipboard API |

## Detection Process

### Application Discovery

The system scans these directories for applications:
- `/Applications/` (user-installed apps)
- `/System/Applications/` (system apps)
- `~/Applications/` (user-specific apps)

### Bundle ID Verification

For each known application, the system:
1. Checks if the app bundle exists at expected paths
2. Verifies the bundle structure
3. Marks the app as available/unavailable

### Dynamic Loading

App availability is checked on-demand when the share dialog opens, ensuring real-time accuracy.

## Share Methods

### URL Schemes

Most modern apps support URL schemes for deep linking:

```typescript
// WhatsApp
whatsapp://send?text=${encodeURIComponent(message)}

// Telegram  
tg://msg_url?text=${encodeURIComponent(message)}

// Mail
mailto:?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(message)}
```

### AppleScript Integration

For apps without URL scheme support, we use AppleScript:

#### Messages App
```applescript
tell application "Messages"
  activate
  set newMessage to make new outgoing message with properties {content:"${message}"}
end tell
```

#### Notes App
```applescript
tell application "Notes"
  activate
  make new note with properties {body:"${message}"}
end tell
```

### Security Considerations

1. **Whitelisted Scripts**: Only predefined AppleScript snippets are executed
2. **Input Sanitization**: All user content is properly escaped
3. **No Arbitrary Execution**: No dynamic script generation or execution
4. **Sandboxing Compatible**: Works within Electron's security constraints

## IPC API

### share:listTargets

Returns available share targets based on installed applications.

**Response:**
```typescript
{
  ok: boolean;
  data?: ShareTarget[];
  error?: string;
}

interface ShareTarget {
  id: string;
  name: string;
  icon: string;
  available: boolean;
}
```

**Example Response:**
```json
{
  "ok": true,
  "data": [
    {
      "id": "mail",
      "name": "Mail",
      "icon": "📧",
      "available": true
    },
    {
      "id": "whatsapp", 
      "name": "WhatsApp",
      "icon": "📱",
      "available": true
    },
    {
      "id": "copy",
      "name": "Copy to Clipboard", 
      "icon": "📋",
      "available": true
    }
  ]
}
```

### share:invoke

Invokes a specific share target with the provided content.

**Parameters:**
- `targetId: string` - The ID of the share target
- `payload: string` - The content to share

**Response:**
```typescript
{
  ok: boolean;
  error?: string;
}
```

## UI Components

### ShareDialog Component

The `ShareDialog` component provides a modern, glass-morphism styled interface:

#### Features:
- **Grid Layout**: Apps displayed in a 2-column grid
- **Visual Feedback**: Loading states and success animations
- **Content Preview**: Shows the invitation content before sharing
- **Responsive Design**: Adapts to different screen sizes
- **Accessibility**: Proper ARIA labels and keyboard navigation

#### Styling:
- **Glass Effect**: `backdrop-blur-xl` with semi-transparent backgrounds
- **Rounded Corners**: `rounded-2xl` for modern appearance
- **Micro-interactions**: Hover effects and loading animations
- **Dark Mode**: Full support for light/dark themes

## Error Handling

### Common Error Scenarios:

1. **App Not Installed**: Target is filtered out from available options
2. **Permission Denied**: Shows appropriate error message
3. **AppleScript Failure**: Falls back to URL schemes where possible
4. **Network Issues**: Affects cloud-based sharing methods

### Error Recovery:

```typescript
try {
  await window.fv.share.invoke(targetId, content);
} catch (error) {
  // Fallback to clipboard
  await window.fv.copyToClipboard(content);
  showToast('Copied to clipboard as fallback');
}
```

## Platform Compatibility

### macOS (Primary)
- ✅ Full AppleScript support
- ✅ Native app detection
- ✅ URL scheme handling
- ✅ Clipboard integration

### Windows (Future)
- 🔄 PowerShell script alternatives
- 🔄 Registry-based app detection
- 🔄 Windows-specific URL schemes

### Linux (Future)
- 🔄 Desktop file parsing
- 🔄 XDG utilities integration
- 🔄 D-Bus messaging

## Performance Optimizations

1. **Lazy Detection**: Apps are only checked when share dialog opens
2. **Caching**: App availability is cached briefly to avoid repeated filesystem checks
3. **Async Operations**: All sharing operations are non-blocking
4. **Minimal Dependencies**: Uses built-in Electron and Node.js APIs where possible

## Deep Link Examples

### Invitation Content Format

```
Join our FamilyVault family group!

Pairing token: ABC123DEF456

QR Code: data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...
```

### URL Scheme Results

#### WhatsApp
```
whatsapp://send?text=Join%20our%20FamilyVault%20family%20group%21%0A%0APairing%20token%3A%20ABC123DEF456
```

#### Mail
```
mailto:?subject=FamilyVault%20Invite&body=Join%20our%20FamilyVault%20family%20group%21%0A%0APairing%20token%3A%20ABC123DEF456
```

## Future Enhancements

1. **Custom App Support**: Allow users to add custom sharing targets
2. **Template System**: Customizable message templates for different apps
3. **Batch Sharing**: Share to multiple targets simultaneously
4. **Analytics**: Track sharing success rates and popular methods
5. **QR Code Integration**: Generate and share QR codes directly
6. **Contact Integration**: Pre-fill recipient information from contacts

## Testing

### Manual Testing Checklist

- [ ] Mail app opens with pre-filled subject and body
- [ ] Messages creates new message with invitation text
- [ ] WhatsApp opens with pre-filled message
- [ ] Telegram opens with pre-filled message
- [ ] Notes creates new note with invitation content
- [ ] Copy to clipboard works and shows confirmation
- [ ] Unavailable apps are filtered out
- [ ] Error handling works for failed operations

### Automated Testing

```typescript
// Example test
describe('Share System', () => {
  it('should detect available apps', async () => {
    const result = await window.fv.share.listTargets();
    expect(result.ok).toBe(true);
    expect(result.data).toContain(
      expect.objectContaining({ id: 'copy', available: true })
    );
  });
});
```