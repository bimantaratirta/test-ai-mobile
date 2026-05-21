import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SecureStorage {
  SecureStorage()
      : _box = const FlutterSecureStorage(
          aOptions: AndroidOptions(encryptedSharedPreferences: true),
          iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
        );

  final FlutterSecureStorage _box;

  Future<String?> read(String key) => _box.read(key: key);
  Future<void> write(String key, String value) =>
      _box.write(key: key, value: value);
  Future<void> delete(String key) => _box.delete(key: key);
  Future<void> deleteAll() => _box.deleteAll();
}
