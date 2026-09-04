import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'documents.dart';

/// Aplikimi si shofer/korrier: automjeti dhe kategoritë. Miratimin e jep Operacionet pasi
/// dokumentet të jenë ngarkuar dhe shqyrtuar (§30–31); deri atëherë turni mbetet i mbyllur.
class ApplyScreen extends StatefulWidget {
  const ApplyScreen({super.key});

  static const categories = ['economy', 'comfort', 'xl', 'taxi'];

  @override
  State<ApplyScreen> createState() => _ApplyScreenState();
}

class _ApplyScreenState extends State<ApplyScreen> {
  final _make = TextEditingController();
  final _model = TextEditingController();
  final _plate = TextEditingController();
  final _color = TextEditingController();
  final _selected = <String>{'economy'};
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    for (final c in [_make, _model, _plate, _color]) {
      c.dispose();
    }
    super.dispose();
  }

  bool get _valid =>
      _make.text.trim().length >= 2 &&
      _model.text.trim().isNotEmpty &&
      _plate.text.trim().length >= 4 &&
      _color.text.trim().isNotEmpty &&
      _selected.isNotEmpty;

  Future<void> _submit() async {
    if (!_valid) {
      setState(() => _error = context.t('errors.validation'));
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    final state = context.read<AppState>();
    try {
      await state.api.applyAsDriver(
        make: _make.text.trim(),
        model: _model.text.trim(),
        plate: _plate.text.trim().toUpperCase(),
        color: _color.text.trim(),
        categories: _selected.toList(),
      );
      await state.refreshDriver();
      if (!mounted) return;
      await Navigator.of(context)
          .pushReplacement(MaterialPageRoute<void>(builder: (_) => const DocumentsScreen()));
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _busy = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('driver.apply.title'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            Text(
              context.t('driver.apply.intro'),
              style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
            ),
            const SizedBox(height: K.s5),
            KSectionHeader(context.t('driver.apply.vehicle')),
            const SizedBox(height: K.s3),
            KField(
              label: context.t('driver.apply.make'),
              controller: _make,
              hint: 'Volkswagen',
              textInputAction: TextInputAction.next,
              onChanged: (_) => setState(() => _error = null),
            ),
            const SizedBox(height: K.s3),
            KField(
              label: context.t('driver.apply.model'),
              controller: _model,
              hint: 'Passat',
              textInputAction: TextInputAction.next,
              onChanged: (_) => setState(() => _error = null),
            ),
            const SizedBox(height: K.s3),
            KField(
              label: context.t('driver.apply.plate'),
              controller: _plate,
              hint: '01-123-AB',
              inputFormatters: [
                FilteringTextInputFormatter.allow(RegExp(r'[A-Za-z0-9\- ]')),
                LengthLimitingTextInputFormatter(12),
              ],
              textInputAction: TextInputAction.next,
              onChanged: (_) => setState(() => _error = null),
            ),
            const SizedBox(height: K.s3),
            KField(
              label: context.t('driver.apply.color'),
              controller: _color,
              hint: context.t('driver.apply.color.hint'),
              textInputAction: TextInputAction.done,
              onChanged: (_) => setState(() => _error = null),
              onSubmitted: (_) => _submit(),
            ),
            const SizedBox(height: K.s5),
            KSectionHeader(context.t('driver.apply.categories')),
            const SizedBox(height: K.s2),
            Text(
              context.t('driver.apply.categories.hint'),
              style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
            ),
            const SizedBox(height: K.s3),
            Wrap(
              spacing: K.s2,
              runSpacing: K.s2,
              children: [
                for (final c in ApplyScreen.categories)
                  FilterChip(
                    label: Text(context.t('ride.category.$c')),
                    selected: _selected.contains(c),
                    onSelected: (on) => setState(() {
                      if (on) {
                        _selected.add(c);
                      } else if (_selected.length > 1) {
                        _selected.remove(c);
                      }
                      _error = null;
                    }),
                  ),
              ],
            ),
            if (_error != null) ...[
              const SizedBox(height: K.s4),
              Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
            ],
            const SizedBox(height: K.s6),
            KButton(
              label: context.t('driver.apply.submit'),
              icon: Icons.arrow_forward,
              busy: _busy,
              onPressed: _busy ? null : _submit,
            ),
            const SizedBox(height: K.s3),
            Text(
              context.t('driver.apply.next'),
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
            ),
          ],
        ),
      ),
    );
  }
}
