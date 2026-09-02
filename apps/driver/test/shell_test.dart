import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_driver/screens/home.dart';
import 'package:krejt_driver/screens/sign_in.dart';
import 'package:krejt_driver/state/app_state.dart';
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
  testWidgets('hyrja e shoferit kërkon numrin e telefonit', (tester) async {
    await tester.pumpWidget(_wrap(const SignInScreen()));
    await tester.pumpAndSettle();
    expect(find.text('Numri i telefonit'), findsOneWidget);
  });

  testWidgets('pa profil të aprovuar nuk shfaqet butoni i hyrjes në punë', (tester) async {
    await tester.pumpWidget(_wrap(const DriverHomeScreen()));
    await tester.pump();
    expect(find.text('Llogaria është në shqyrtim'), findsOneWidget);
    expect(find.text('Fillo punën'), findsNothing);
  });

  testWidgets('shenja e gjendjes nis si jashtë pune', (tester) async {
    await tester.pumpWidget(_wrap(const DriverHomeScreen()));
    await tester.pump();
    expect(find.text('Jashtë pune'), findsOneWidget);
    expect(find.byType(KBadge), findsOneWidget);
  });
}
