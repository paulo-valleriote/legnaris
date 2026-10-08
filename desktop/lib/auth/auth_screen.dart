import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/tokens.dart';

/// The unauthenticated surface: one screen with a Log in / Register mode
/// switch, submitting through [AuthSession] (FE6-03 … FE6-05, FE6-21).
///
/// A failure renders the mapped copy inline and keeps the entered fields, so
/// the next activation is a clean retry without retyping. The primary action
/// lives in a fixed footer and disables while the session is authenticating,
/// which is the in-flight signal for both login and register.
class AuthScreen extends ConsumerStatefulWidget {
  const AuthScreen({super.key});

  @override
  ConsumerState<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends ConsumerState<AuthScreen> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  final _usernameController = TextEditingController();

  /// False renders the login form; true renders the registration form.
  bool _registerMode = false;

  /// Mapped copy of the last failure; null until one fails.
  String? _errorMessage;

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    _usernameController.dispose();
    super.dispose();
  }

  /// Switches mode and drops a stale error so a login failure never shows on
  /// the registration form (and vice versa). Entered values are kept.
  void _setMode(bool register) {
    if (_registerMode == register) return;
    setState(() {
      _registerMode = register;
      _errorMessage = null;
    });
  }

  /// Submits through the session; a failure keeps the screen and renders the
  /// mapped copy inline while the controllers keep the entered values
  /// (FE6-05, FE6-21). A success flips the session and the gate replaces this
  /// screen.
  Future<void> _submit() async {
    if (ref.read(authSessionProvider) is AuthAuthenticating) return;
    setState(() => _errorMessage = null);
    final session = ref.read(authSessionProvider.notifier);
    try {
      if (_registerMode) {
        await session.register(
          email: _emailController.text,
          username: _usernameController.text,
          password: _passwordController.text,
        );
      } else {
        await session.login(
          email: _emailController.text,
          password: _passwordController.text,
        );
      }
    } catch (error) {
      if (!mounted) return;
      setState(() => _errorMessage = failureCopy(error));
    }
  }

  @override
  Widget build(BuildContext context) {
    final localizations = AppLocalizations.of(context)!;
    final pending = ref.watch(authSessionProvider) is AuthAuthenticating;
    return Scaffold(
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: SingleChildScrollView(
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(
                    maxWidth: AppSizes.authFormMaxWidth,
                  ),
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.xl),
                    child: _form(context, localizations, pending),
                  ),
                ),
              ),
            ),
          ),
          const Divider(),
          _footer(context, localizations, pending),
        ],
      ),
    );
  }

  /// The form column: app identity, the mode switch, the fields and the
  /// inline mapped error.
  Widget _form(
    BuildContext context,
    AppLocalizations localizations,
    bool pending,
  ) {
    final theme = Theme.of(context);
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(localizations.appTitle, style: theme.textTheme.titleLarge),
        const SizedBox(height: AppSpacing.lg),
        Row(
          children: [
            _modeTab(
              context,
              key: const Key('auth-mode-login'),
              label: localizations.authModeLogin,
              active: !_registerMode,
              onTap: () => _setMode(false),
            ),
            const SizedBox(width: AppSpacing.sm),
            _modeTab(
              context,
              key: const Key('auth-mode-register'),
              label: localizations.authModeRegister,
              active: _registerMode,
              onTap: () => _setMode(true),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.lg),
        TextField(
          key: const Key('auth-email'),
          controller: _emailController,
          enabled: !pending,
          decoration: InputDecoration(labelText: localizations.authEmailLabel),
        ),
        const SizedBox(height: AppSpacing.md),
        TextField(
          key: const Key('auth-password'),
          controller: _passwordController,
          enabled: !pending,
          obscureText: true,
          decoration: InputDecoration(
            labelText: localizations.authPasswordLabel,
          ),
        ),
        if (_registerMode) ...[
          const SizedBox(height: AppSpacing.md),
          TextField(
            key: const Key('auth-username'),
            controller: _usernameController,
            enabled: !pending,
            decoration: InputDecoration(
              labelText: localizations.authUsernameLabel,
            ),
          ),
        ],
        if (_errorMessage != null) ...[
          const SizedBox(height: AppSpacing.md),
          Text(
            _errorMessage!,
            key: const Key('auth-error'),
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.error,
            ),
          ),
        ],
      ],
    );
  }

  /// One tab of the mode switch. The active tab reads in the primary color
  /// and heavier weight; the inactive one stays muted, and both remain
  /// clickable so the switch never looks disabled.
  Widget _modeTab(
    BuildContext context, {
    required Key key,
    required String label,
    required bool active,
    required VoidCallback onTap,
  }) {
    final theme = Theme.of(context);
    return TextButton(
      key: key,
      onPressed: onTap,
      style: TextButton.styleFrom(
        foregroundColor: active
            ? theme.colorScheme.primary
            : theme.colorScheme.onSurfaceVariant,
      ),
      child: Text(
        label,
        style: theme.textTheme.bodyMedium?.copyWith(
          fontWeight: active ? FontWeight.w600 : null,
        ),
      ),
    );
  }

  /// The fixed footer bar: progress while pending and the single primary
  /// action, disabled until the request settles (FE6-05).
  Widget _footer(
    BuildContext context,
    AppLocalizations localizations,
    bool pending,
  ) {
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.xl,
        vertical: AppSpacing.md,
      ),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(
            maxWidth: AppSizes.authFormMaxWidth,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (pending) ...[
                const LinearProgressIndicator(key: Key('auth-progress')),
                const SizedBox(height: AppSpacing.sm),
              ],
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  key: const Key('auth-submit'),
                  onPressed: pending ? null : _submit,
                  child: Text(
                    _registerMode
                        ? localizations.authSubmitRegister
                        : localizations.authSubmitLogin,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
