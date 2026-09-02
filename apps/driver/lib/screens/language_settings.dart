import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Ndërrimi i gjuhës pas kyçjes: ruhet në pajisje dhe sinkronizohet me profilin,
/// që njoftimet të vijnë në të njëjtën gjuhë si aplikacioni (§2, §29).
class DriverLanguageScreen extends StatelessWidget {
  const DriverLanguageScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('account.language'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(K.s5),
          children: [
            for (final code in const ['sq', 'en', 'de'])
              Padding(
                padding: const EdgeInsets.only(bottom: K.s2),
                child: KCard(
                  onTap: () => state.setLocale(code),
                  highlight: state.locale == code,
                  child: SizedBox(
                    height: K.minTap - K.s4,
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            KL10n.languageName(code),
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: state.locale == code ? FontWeight.w700 : FontWeight.w500,
                              color: state.locale == code ? K.text : K.textDim,
                            ),
                          ),
                        ),
                        Icon(
                          state.locale == code
                              ? Icons.radio_button_checked
                              : Icons.radio_button_unchecked,
                          size: 22,
                          color: state.locale == code ? K.brand400 : K.line2,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
