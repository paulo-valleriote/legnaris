import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'template_detail_provider.g.dart';

/// Render state of the template detail loaded by the creation surface
/// (TPL-01, TPL-04).
class TemplateDetailState {
  const TemplateDetailState({
    this.template,
    this.isLoading = false,
    this.errorMessage,
  });

  /// Decoded detail (id, name, `fields` intact) after a success; null before.
  final TemplateDetail? template;

  /// A detail request is in flight (first load or Retry).
  final bool isLoading;

  /// Resolved UI copy of the last failure; null after a success.
  final String? errorMessage;
}

/// Detail of one template, fetched on demand for the creation surface
/// (TPL-01).
///
/// The family is autoDispose: the creation dialog disposes it when it closes,
/// so every reopen fetches again with no cached reuse (TPL-15) and a new
/// template id can never inherit a previous summary (TPL-04). Nothing is ever
/// persisted.
@riverpod
class TemplateDetailController extends _$TemplateDetailController {
  bool _inFlight = false;

  @override
  TemplateDetailState build(String templateId) {
    ref.watch(apiClientProvider);
    Future.microtask(load);
    return const TemplateDetailState(isLoading: true);
  }

  /// Loads the detail unless a request is already in flight (single-flight, so
  /// Retry never doubles up). A failure keeps the mapped copy in the state for
  /// the dialog to render inline with Retry (TPL-07 … TPL-10). The mounted
  /// guard drops the result when the dialog closed mid-flight (TPL-14).
  Future<void> load() async {
    if (_inFlight) return;
    _inFlight = true;
    state = const TemplateDetailState(isLoading: true);
    try {
      final template = await ref
          .read(apiClientProvider)
          .getTemplate(templateId);
      if (!ref.mounted) return;
      state = TemplateDetailState(template: template);
    } catch (error) {
      if (!ref.mounted) return;
      state = TemplateDetailState(errorMessage: failureCopy(error));
    } finally {
      _inFlight = false;
    }
  }
}
