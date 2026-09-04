import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_screens/krejt_screens.dart';

Widget _wrap(Widget child, {String locale = 'sq'}) => MaterialApp(
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
);

/// Klienti nuk lidhet me asgjë: këto teste nuk duan rrjet, vetëm strukturën e ekranit.
KrejtApi _api() => KrejtApi(
  config: const ApiConfig(
    baseUrl: 'http://127.0.0.1:1',
    appId: 'customer',
    platform: 'android',
    appVersion: 'test',
  ),
  session: Session(store: MemoryStore()),
);

void main() {
  // Dokumentet ligjore i kërkojnë të dy dyqanet për të dy aplikacionet. Nëse dikush e heq njërën
  // hyrje, ky test bie para se ta zbulojë shqyrtuesi.
  testWidgets('ekrani ligjor ofron të dy dokumentet', (tester) async {
    await tester.pumpWidget(_wrap(LegalScreen(api: _api(), locale: 'sq')));
    await tester.pumpAndSettle();
    expect(find.text('Kushtet e përdorimit'), findsOneWidget);
    expect(find.text('Politika e privatësisë'), findsOneWidget);
  });

  testWidgets('ekrani ligjor flet gjuhën e zgjedhur', (tester) async {
    await tester.pumpWidget(
      _wrap(
        LegalScreen(api: _api(), locale: 'de'),
        locale: 'de',
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Nutzungsbedingungen'), findsOneWidget);
    expect(find.text('Datenschutzerklärung'), findsOneWidget);
  });

  // Siguria rri brenda listës me qëllim: pa të, një problem sigurie nuk do të kishte fare rrugë
  // nga aplikacioni.
  test('kategoritë e mbështetjes përfshijnë sigurinë dhe nuk përsëriten', () {
    expect(supportCategories, contains('safety'));
    expect(supportCategories.toSet().length, supportCategories.length);
  });

  // Një tiketë flet për një gjë të vetme; serveri e refuzon nëse jepen dy.
  test('subjekti i tiketës mban një referencë të vetme', () {
    const s = TicketSubject(category: 'order', orderId: 'o1');
    expect(s.orderId, 'o1');
    expect(s.parcelId, isNull);
    expect(s.requestId, isNull);
    expect(s.copyWith(parcelId: 'p1').orderId, 'o1');
  });
}
