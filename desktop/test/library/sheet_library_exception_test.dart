import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/library/sheet_library_exception.dart';

/// Unit tests for the typed local sheet library failure (LIB-11, LIB-20).
void main() {
  test('Happy path: carries the writeFailed kind and its message (LIB-11)', () {
    const error = SheetLibraryException(
      SheetLibraryErrorKind.writeFailed,
      'disk full',
    );

    expect(error.kind, SheetLibraryErrorKind.writeFailed);
    expect(error.message, 'disk full');
  });

  test('Happy path: carries the invalidEntry kind and its message (LIB-20)', () {
    const error = SheetLibraryException(
      SheetLibraryErrorKind.invalidEntry,
      'missing name',
    );

    expect(error.kind, SheetLibraryErrorKind.invalidEntry);
    expect(error.message, 'missing name');
  });

  test('Happy path: toString includes the kind and the message', () {
    const error = SheetLibraryException(
      SheetLibraryErrorKind.writeFailed,
      'disk full',
    );

    expect(error.toString(), contains('writeFailed'));
    expect(error.toString(), contains('disk full'));
  });
}
