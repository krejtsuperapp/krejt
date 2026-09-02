import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import 'screens/blocked.dart';
import 'screens/boot.dart';
import 'screens/home.dart';
import 'screens/language.dart';
import 'screens/sign_in.dart';
import 'state/app_state.dart';

class KrejtDriverApp extends StatelessWidget {
  const KrejtDriverApp({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    return MaterialApp(
      title: 'KREJT Driver',
      debugShowCheckedModeBanner: false,
      theme: krejtTheme(),
      locale: Locale(state.locale),
      supportedLocales: kSupportedLocales,
      localizationsDelegates: const [
        KL10n.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      home: const _Gate(),
    );
  }
}

class _Gate extends StatelessWidget {
  const _Gate();

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    switch (state.phase) {
      case BootPhase.starting:
        return const BootScreen();
      case BootPhase.blocked:
        return const BlockedScreen();
      case BootPhase.needsLanguage:
        return const LanguageScreen();
      case BootPhase.signedOut:
        return const SignInScreen();
      case BootPhase.ready:
        return const DriverHomeScreen();
      case BootPhase.failed:
        return const BootScreen(showRetry: true);
    }
  }
}
