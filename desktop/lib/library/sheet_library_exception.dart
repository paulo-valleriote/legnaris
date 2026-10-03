/// Typed failures raised by the local sheet library store.
library;

/// The failure classes the store and its callers distinguish: a persistence
/// failure versus a rejected entry (LIB-11, LIB-20).
enum SheetLibraryErrorKind { writeFailed, invalidEntry }

/// A local sheet library failure carrying its [kind] and a message, mirroring
/// the kind + message shape of the API's `ApiDomainException`.
class SheetLibraryException implements Exception {
  const SheetLibraryException(this.kind, this.message);

  final SheetLibraryErrorKind kind;

  final String message;

  @override
  String toString() => 'SheetLibraryException(${kind.name}): $message';
}
