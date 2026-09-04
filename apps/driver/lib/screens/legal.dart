import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Kushtet dhe privatësia. Teksti vjen nga serveri, në gjuhën e zgjedhur: një ndreqje e tij nuk
/// duhet të presë një version të ri të aplikacionit, dhe i njëjti dokument shërbehet edhe si faqe
/// publike për dyqanet e aplikacioneve.
class LegalScreen extends StatelessWidget {
  const LegalScreen({super.key});

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: K.bg,
    appBar: AppBar(title: Text(context.t('account.legal'))),
    body: SafeArea(
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
        children: [
          _Entry(
            icon: Icons.gavel_outlined,
            label: context.t('legal.terms'),
            onTap: () => _open(context, 'terms'),
          ),
          const SizedBox(height: K.s2),
          _Entry(
            icon: Icons.shield_outlined,
            label: context.t('legal.privacy'),
            onTap: () => _open(context, 'privacy'),
          ),
        ],
      ),
    ),
  );

  static void _open(BuildContext context, String doc) =>
      Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => LegalDocScreen(doc: doc)));
}

class _Entry extends StatelessWidget {
  const _Entry({required this.icon, required this.label, required this.onTap});

  final IconData icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => KCard(
    onTap: onTap,
    padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s4),
    child: Row(
      children: [
        Icon(icon, size: 20, color: K.textDim),
        const SizedBox(width: K.s3),
        Expanded(
          child: Text(
            label,
            style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
          ),
        ),
        const Icon(Icons.chevron_right, size: 20, color: K.line2),
      ],
    ),
  );
}

/// Një dokument i vetëm. Kërkon rrjet: teksti ligjor duhet të jetë ai i çastit, jo një kopje e
/// vjetruar e paketuar me aplikacionin.
class LegalDocScreen extends StatefulWidget {
  const LegalDocScreen({super.key, required this.doc});

  final String doc;

  @override
  State<LegalDocScreen> createState() => _LegalDocScreenState();
}

class _LegalDocScreenState extends State<LegalDocScreen> {
  LegalDocument? _doc;
  ApiError? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    final state = context.read<AppState>();
    try {
      final d = await state.api.legalDocument(widget.doc, lang: state.locale);
      if (!mounted) return;
      setState(() {
        _doc = d;
        _error = null;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final doc = _doc;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(
          doc?.title ?? context.t(widget.doc == 'terms' ? 'legal.terms' : 'legal.privacy'),
        ),
      ),
      body: SafeArea(
        child: _loading
            ? const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 20, count: 12))
            : doc == null
            ? Padding(
                padding: const EdgeInsets.all(K.s5),
                child: KError(
                  message: context.tError(_error?.messageKey ?? 'errors.internal'),
                  retryLabel: context.t('common.retry'),
                  onRetry: _load,
                ),
              )
            : ListView(
                padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s8),
                children: [
                  Text(
                    context.t('legal.updated', {'date': _date(doc.updated)}),
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
                  const SizedBox(height: K.s5),
                  for (final s in doc.sections) ...[
                    Text(
                      s.heading,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                        height: 1.3,
                      ),
                    ),
                    const SizedBox(height: K.s2),
                    for (final p in s.paragraphs)
                      Padding(
                        padding: EdgeInsets.only(bottom: K.s2, left: p.startsWith('• ') ? K.s3 : 0),
                        child: Text(
                          p,
                          style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.6),
                        ),
                      ),
                    const SizedBox(height: K.s4),
                  ],
                ],
              ),
      ),
    );
  }

  /// 2026-09-04 → 4.9.2026, si i shkruhen datat në Kosovë.
  static String _date(String iso) {
    final parts = iso.split('-');
    if (parts.length != 3) return iso;
    final d = int.tryParse(parts[2]), m = int.tryParse(parts[1]);
    if (d == null || m == null) return iso;
    return '$d.$m.${parts[0]}';
  }
}
