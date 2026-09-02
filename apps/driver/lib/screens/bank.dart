import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Kontrolli mod-97 i IBAN-it (ISO 13616). Bëhet edhe këtu që shoferi ta shohë gabimin
/// para se ta dërgojë; serveri e përsërit gjithsesi, sepse ai vendos (§18).
bool isValidIban(String raw) {
  final iban = raw.replaceAll(RegExp(r'\s'), '').toUpperCase();
  if (iban.length < 15 || iban.length > 34) return false;
  if (!RegExp(r'^[A-Z]{2}[0-9]{2}[A-Z0-9]+$').hasMatch(iban)) return false;

  final rearranged = iban.substring(4) + iban.substring(0, 4);
  var remainder = 0;
  for (final ch in rearranged.split('')) {
    final code = ch.codeUnitAt(0);
    final value = code >= 65 && code <= 90 ? '${code - 55}' : ch;
    for (final digit in value.split('')) {
      remainder = (remainder * 10 + int.parse(digit)) % 97;
    }
  }
  return remainder == 1;
}

/// Llogaria bankare për pagesat javore. IBAN-i shfaqet i maskuar pasi ruhet:
/// aplikacioni nuk e mban të plotë as në ekran as në memorie pas ruajtjes (§18, §57).
class BankAccountScreen extends StatefulWidget {
  const BankAccountScreen({super.key, this.current});

  final BankAccount? current;

  @override
  State<BankAccountScreen> createState() => _BankAccountScreenState();
}

class _BankAccountScreenState extends State<BankAccountScreen> {
  late final TextEditingController _holder = TextEditingController(
    text: widget.current?.holderName ?? '',
  );
  final _iban = TextEditingController();

  bool _busy = false;
  String? _ibanError;
  String? _error;

  @override
  void dispose() {
    _holder.dispose();
    _iban.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final iban = _iban.text.replaceAll(' ', '').toUpperCase();
    if (!isValidIban(iban)) {
      setState(() => _ibanError = context.t('driver.bank.invalid'));
      return;
    }
    setState(() {
      _busy = true;
      _ibanError = null;
      _error = null;
    });
    final saved = context.t('account.saved');
    final messenger = ScaffoldMessenger.of(context);
    try {
      await context.read<AppState>().api.saveBankAccount(
        holderName: _holder.text.trim(),
        iban: iban,
      );
      if (!mounted) return;
      _iban.clear();
      messenger.showSnackBar(SnackBar(content: Text(saved)));
      Navigator.of(context).pop(true);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = context.tError(e.messageKey));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final current = widget.current;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('driver.bank'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(K.s5),
          children: [
            if (current != null) ...[
              KCard(
                child: Column(
                  children: [
                    KRow(context.t('driver.bank.iban'), current.ibanMasked),
                    KRow(context.t('driver.bank.holder'), current.holderName),
                  ],
                ),
              ),
              const SizedBox(height: K.s5),
            ],
            KField(
              label: context.t('driver.bank.holder'),
              controller: _holder,
              textInputAction: TextInputAction.next,
              autofillHints: const [AutofillHints.name],
            ),
            const SizedBox(height: K.s4),
            KField(
              label: context.t('driver.bank.iban'),
              controller: _iban,
              hint: 'XK05 1212 0123 4567 8906',
              error: _ibanError,
              maxLength: 34,
              inputFormatters: [
                FilteringTextInputFormatter.allow(RegExp(r'[A-Za-z0-9 ]')),
                LengthLimitingTextInputFormatter(34),
              ],
              onChanged: (_) => setState(() => _ibanError = null),
            ),
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
