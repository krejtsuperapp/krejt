import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_customer/screens/blocked.dart';
import 'package:krejt_customer/screens/onboarding.dart';
import 'package:krejt_customer/screens/sign_in.dart';
import 'package:krejt_customer/state/app_state.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

/// Pa `locale` MaterialApp-i e ndjek gjuhën e gjendjes (si aplikacioni i vërtetë); me `locale`
/// të dhënë, teksti ngulet aty për testet e përkthimit.
Widget _wrap(Widget child, {String? locale}) => ChangeNotifierProvider(
  create: (_) => AppState(),
  child: Consumer<AppState>(
    builder: (_, state, _) => MaterialApp(
      theme: krejtTheme(),
      locale: Locale(locale ?? state.locale),
      supportedLocales: kSupportedLocales,
      localizationsDelegates: const [
        KL10n.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      home: child,
    ),
  ),
);

void main() {
  testWidgets('hyrja nis në shqip, me ndërruesin SQ/EN/DE dhe butonin Fillo', (tester) async {
    tester.view.physicalSize = const Size(1080, 2280);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.resetPhysicalSize);
    await tester.pumpWidget(_wrap(const OnboardingScreen()));
    await tester.pump();
    expect(find.byType(KLangSwitch), findsOneWidget);
    expect(find.text('SQ'), findsOneWidget);
    expect(find.text('EN'), findsOneWidget);
    expect(find.text('DE'), findsOneWidget);
    expect(find.text('Fillo'), findsOneWidget);
    expect(find.byType(KWordmark), findsOneWidget);
  });

  testWidgets('prekja e DE e ndërron tekstin në çast', (tester) async {
    tester.view.physicalSize = const Size(1080, 2280);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.resetPhysicalSize);
    await tester.pumpWidget(_wrap(const OnboardingScreen()));
    await tester.pump();
    await tester.tap(find.text('DE'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.text('Loslegen'), findsOneWidget);
    expect(find.text('Fillo'), findsNothing);
  });

  testWidgets('hyrja kërkon numrin para kodit', (tester) async {
    await tester.pumpWidget(_wrap(const SignInScreen()));
    await tester.pumpAndSettle();
    expect(find.text('Numri i telefonit'), findsOneWidget);
    expect(find.byType(KOtpField), findsNothing);
  });

  testWidgets('numri i shkurtër nuk dërgon kërkesë dhe tregon gabimin', (tester) async {
    await tester.pumpWidget(_wrap(const SignInScreen()));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).first, '44');
    await tester.tap(find.text('Vazhdo'));
    await tester.pump();
    expect(find.text('Numri nuk duket i saktë'), findsOneWidget);
  });

  testWidgets('muri i bllokimit shpjegon gjendjen pa buton që s\'bën gjë', (tester) async {
    await tester.pumpWidget(_wrap(const BlockedScreen()));
    await tester.pumpAndSettle();
    expect(find.text('Kërkohet përditësimi'), findsOneWidget);
    expect(find.byType(KButton), findsNothing);
  });

  testWidgets('teksti ndërron gjuhë pa ndryshuar strukturën', (tester) async {
    await tester.pumpWidget(_wrap(const SignInScreen(), locale: 'de'));
    await tester.pumpAndSettle();
    expect(find.text('Telefonnummer'), findsOneWidget);
  });
}
