import 'dart:io';

import 'package:flutter/material.dart';
import 'package:image_cropper/image_cropper.dart';
import 'package:image_picker/image_picker.dart';
import 'package:go_router/go_router.dart';

/// Returned as a route argument to /compose so the compose screen can show
/// a preview thumbnail and submit the file path on Generate.
class CaptureResult {
  CaptureResult(this.file);
  final File file;
}

class CaptureScreen extends StatefulWidget {
  const CaptureScreen({super.key});
  @override
  State<CaptureScreen> createState() => _CaptureScreenState();
}

class _CaptureScreenState extends State<CaptureScreen> {
  final _picker = ImagePicker();
  bool _busy = false;

  Future<void> _pick(ImageSource source) async {
    setState(() => _busy = true);
    try {
      final picked = await _picker.pickImage(
        source: source,
        imageQuality: 92,
        maxWidth: 3000,
        maxHeight: 3000,
      );
      if (picked == null) return;

      final cropped = await ImageCropper().cropImage(
        sourcePath: picked.path,
        aspectRatio: const CropAspectRatio(ratioX: 4, ratioY: 3),
        uiSettings: [
          AndroidUiSettings(
            toolbarTitle: 'Crop empty room',
            lockAspectRatio: true,
          ),
          IOSUiSettings(
            title: 'Crop empty room',
            aspectRatioLockEnabled: true,
          ),
        ],
      );
      if (cropped == null) return;

      if (!mounted) return;
      context.go('/compose', extra: CaptureResult(File(cropped.path)));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Capture')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              FilledButton.icon(
                icon: const Icon(Icons.camera_alt),
                label: const Text('Take a photo'),
                onPressed: _busy ? null : () => _pick(ImageSource.camera),
              ),
              const SizedBox(height: 16),
              OutlinedButton.icon(
                icon: const Icon(Icons.photo_library),
                label: const Text('Choose from library'),
                onPressed: _busy ? null : () => _pick(ImageSource.gallery),
              ),
              if (_busy) ...[
                const SizedBox(height: 24),
                const CircularProgressIndicator(),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
