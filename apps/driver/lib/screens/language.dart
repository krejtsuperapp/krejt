import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Ekrani i parë që sheh një përdorues i ri: gjuha. Shqipja është e para dhe e parazgjedhur (§2).
class LanguageScreen extends StatefulWidget {
  const LanguageScreen({super.key});

  @override
  State<LanguageScreen> createState() => _LanguageScreenState();
}

class _LanguageScreenState extends State<LanguageScreen> {
  String _selected = 'sq';
  bool _busy = false;

  Future<void> _continue() async {
    setState(() => _busy = true);
    await context.read<AppState>().completeLanguage(_selected);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(K.s5),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: K.s8),
              Text(
                context.t('onboarding.title'),
                style: const TextStyle(
                  fontSize: 28,
                  fontWeight: FontWeight.w800,
                  color: K.text,
                  height: 1.15,
                ),
              ),
              const SizedBox(height: K.s2),
              Text(
                context.t('onboarding.subtitle'),
                style: const TextStyle(fontSize: 15, color: K.textDim, height: 1.45),
              ),
              const SizedBox(height: K.s8),
              Text(
                context.t('onboarding.language'),
                style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: K.muted,
                  letterSpacing: 0.6,
                ),
              ),
              const SizedBox(height: K.s3),
              for (final code in const ['sq', 'en', 'de'])
                Padding(
                  padding: const EdgeInsets.only(bottom: K.s2),
                  child: _LanguageTile(
                    code: code,
                    selected: _selected == code,
                    onTap: () => setState(() => _selected = code),
                  ),
                ),
              const Spacer(),
              KButton(
                label: context.t('common.continue'),
                busy: _busy,
                onPressed: _busy ? null : _continue,
              ),
              const SizedBox(height: K.s4),
            ],
          ),
        ),
      ),
    );
  }
}

class _LanguageTile extends StatelessWidget {
  const _LanguageTile({required this.code, required this.selected, required this.onTap});

  final String code;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return KCard(
      onTap: onTap,
      highlight: selected,
      child: Row(
        children: [
          Expanded(
            child: Text(
              KL10n.languageName(code),
              style: TextStyle(
                fontSize: 16,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                color: selected ? K.text : K.textDim,
              ),
            ),
          ),
          Icon(
            selected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
            color: selected ? K.brand400 : K.line2,
            size: 22,
          ),
        ],
      ),
    );
  }
}
