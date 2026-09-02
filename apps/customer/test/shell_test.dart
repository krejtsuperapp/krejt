import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_customer/screens/blocked.dart';
import 'package:krejt_customer/screens/language.dart';
import 'package:krejt_customer/screens/sign_in.dart';
import 'package:krejt_customer/state/app_state.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

Widget _wrap(Widget child, {String locale = 'sq'}) => ChangeNotifierProvider(
  create: (_) => AppState(),
  child: MaterialApp(
    theme: krejtTheme(),
    locale: Locale(locale),
    supportedLocales: kSupportedLocales,
    localizationsDelegates: const [
      KL10n.delegate,
      GlobalMaterialLocalizations.delegate,
      GlobalWidgetsLocalizations.delegate,
      GlobalCupertinoLocalizations.delegate,
    ],
    home: child,
  ),
);

void main() {
  testWidgets('ekrani i gjuhës nis me shqipen të zgjedhur', (tester) async {
    await tester.pumpWidget(_wrap(const LanguageScreen()));
    await tester.pumpAndSettle();
    expect(find.text('Shqip'), findsOneWidget);
    expect(find.text('English'), findsOneWidget);
    expect(find.text('Deutsch'), findsOneWidget);
    expect(find.byIcon(Icons.radio_button_checked), findsOneWidget);
  });

  testWidgets('zgjedhja e gjuhës lëviz shenjën te rreshti i prekur', (tester) async {
    await tester.pumpWidget(_wrap(const LanguageScreen()));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Deutsch'));
    await tester.pump();
    final checked = tester.widgetList<Icon>(find.byIcon(Icons.radio_button_checked));
    expect(checked.length, 1);
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
