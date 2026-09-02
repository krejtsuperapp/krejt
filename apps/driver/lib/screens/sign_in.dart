import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Kyçja me numër telefoni dhe kod njëpërdorimësh (§16). Aplikacioni nuk mban fjalëkalime
/// dhe nuk e di nëse numri ekziston — serveri përgjigjet njësoj në të dyja rastet.
class SignInScreen extends StatefulWidget {
  const SignInScreen({super.key});

  @override
  State<SignInScreen> createState() => _SignInScreenState();
}

class _SignInScreenState extends State<SignInScreen> {
  final _phone = TextEditingController();

  bool _codeSent = false;
  bool _busy = false;
  String? _phoneError;
  String? _codeError;
  int _resendIn = 0;
  Timer? _timer;

  @override
  void dispose() {
    _timer?.cancel();
    _phone.dispose();
    super.dispose();
  }

  /// Numri shkruhet lokalisht (44 123 456) dhe dërgohet gjithmonë në formatin ndërkombëtar.
  String get _e164 {
    final digits = _phone.text.replaceAll(RegExp(r'\D'), '');
    if (digits.startsWith('383')) return '+$digits';
    if (digits.startsWith('0')) return '+383${digits.substring(1)}';
    return '+383$digits';
  }

  bool get _phoneValid {
    final digits = _phone.text.replaceAll(RegExp(r'\D'), '');
    return digits.length >= 8 && digits.length <= 12;
  }

  void _startResendTimer() {
    _resendIn = 60;
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) return t.cancel();
      setState(() => _resendIn = _resendIn - 1);
      if (_resendIn <= 0) t.cancel();
    });
  }

  Future<void> _requestCode() async {
    if (!_phoneValid) {
      setState(() => _phoneError = context.t('auth.phone.invalid'));
      return;
    }
    setState(() {
      _busy = true;
      _phoneError = null;
    });
    try {
      await context.read<AppState>().api.requestOtp(_e164);
      if (!mounted) return;
      setState(() {
        _codeSent = true;
        _codeError = null;
      });
      _startResendTimer();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _phoneError = context.tError(e.messageKey));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify(String code) async {
    setState(() {
      _busy = true;
      _codeError = null;
    });
    final state = context.read<AppState>();
    try {
      final me = await state.api.verifyOtp(phone: _e164, code: code);
      await state.onSignedIn(me);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _codeError = context.tError(e.messageKey));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        leading: _codeSent
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: () => setState(() => _codeSent = false),
              )
            : null,
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(K.s5),
          child: _codeSent ? _codeStep(context) : _phoneStep(context),
        ),
      ),
    );
  }

  Widget _phoneStep(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Text(
        context.t('auth.phone.title'),
        style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w800, color: K.text),
      ),
      const SizedBox(height: K.s2),
      Text(
        context.t('auth.phone.subtitle'),
        style: const TextStyle(fontSize: 15, color: K.textDim),
      ),
      const SizedBox(height: K.s6),
      KField(
        label: context.t('auth.phone.label'),
        controller: _phone,
        hint: context.t('auth.phone.hint'),
        error: _phoneError,
        prefix: const Text('+383', style: TextStyle(color: K.textDim, fontSize: 16)),
        keyboardType: TextInputType.phone,
        textInputAction: TextInputAction.done,
        autofillHints: const [AutofillHints.telephoneNumber],
        inputFormatters: [
          FilteringTextInputFormatter.digitsOnly,
          LengthLimitingTextInputFormatter(12),
        ],
        autofocus: true,
        onChanged: (_) => setState(() => _phoneError = null),
        onSubmitted: (_) => _requestCode(),
      ),
      const SizedBox(height: K.s5),
      KButton(
        label: context.t('common.continue'),
        busy: _busy,
        onPressed: _busy ? null : _requestCode,
      ),
      const SizedBox(height: K.s4),
      Text(
        context.t('auth.terms'),
        textAlign: TextAlign.center,
        style: const TextStyle(fontSize: 12, color: K.muted, height: 1.45),
      ),
    ],
  );

  Widget _codeStep(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Text(
        context.t('auth.otp.title'),
        style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w800, color: K.text),
      ),
      const SizedBox(height: K.s2),
      Text(
        context.t('auth.otp.subtitle', {'phone': _e164}),
        style: const TextStyle(fontSize: 15, color: K.textDim),
      ),
      const SizedBox(height: K.s6),
      KOtpField(error: _codeError, enabled: !_busy, onCompleted: _verify),
      const SizedBox(height: K.s5),
      Center(
        child: _resendIn > 0
            ? Text(
                context.t('auth.otp.resend.in', {'s': '$_resendIn'}),
                style: const TextStyle(fontSize: 14, color: K.muted),
              )
            : KTextLink(label: context.t('auth.otp.resend'), onPressed: _requestCode),
      ),
    ],
  );
}
