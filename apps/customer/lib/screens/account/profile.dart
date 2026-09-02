import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Profili: vetëm emri dhe email-i ndryshohen këtu. Numri i telefonit është identiteti
/// i llogarisë dhe ndryshohet me verifikim, jo me një fushë teksti (§16).
class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  late final TextEditingController _name;
  late final TextEditingController _email;
  bool _busy = false;
  String? _error;
  Map<String, String> _fieldErrors = const {};

  @override
  void initState() {
    super.initState();
    final me = context.read<AppState>().me;
    _name = TextEditingController(text: me?.fullName ?? '');
    _email = TextEditingController(text: me?.email ?? '');
  }

  @override
  void dispose() {
    _name.dispose();
    _email.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() {
      _busy = true;
      _error = null;
      _fieldErrors = const {};
    });
    final state = context.read<AppState>();
    final messenger = ScaffoldMessenger.of(context);
    final saved = context.t('account.saved');
    try {
      await state.saveProfile(fullName: _name.text.trim(), email: _email.text.trim());
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(saved)));
      Navigator.of(context).pop();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.fields.isEmpty ? context.tError(e.messageKey) : null;
        _fieldErrors = e.fields;
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final me = context.watch<AppState>().me;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('account.profile'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(K.s5),
          children: [
            KField(
              label: context.t('account.name'),
              controller: _name,
              error: _fieldErrors['full_name'] == null ? null : context.t('errors.validation'),
              textInputAction: TextInputAction.next,
              autofillHints: const [AutofillHints.name],
              maxLength: 60,
            ),
            const SizedBox(height: K.s4),
            KField(
              label: context.t('account.email'),
              controller: _email,
              error: _fieldErrors['email'] == null ? null : context.t('errors.validation'),
              keyboardType: TextInputType.emailAddress,
              autofillHints: const [AutofillHints.email],
            ),
            const SizedBox(height: K.s4),
            KCard(child: KRow(context.t('auth.phone.label'), me?.phone ?? '—')),
            if (_error != null) ...[
              const SizedBox(height: K.s3),
              Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
            ],
            const SizedBox(height: K.s6),
            KButton(label: context.t('common.save'), busy: _busy, onPressed: _busy ? null : _save),
          ],
        ),
      ),
    );
  }
}
