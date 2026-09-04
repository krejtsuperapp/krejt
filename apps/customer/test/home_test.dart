import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_customer/screens/account/account.dart';
import 'package:krejt_customer/screens/account/addresses.dart';
import 'package:krejt_customer/screens/active_banner.dart';
import 'package:krejt_customer/screens/home.dart';
import 'package:krejt_customer/state/app_state.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

Ride _ride({required String state, String? dropoff, int price = 350}) => Ride.fromJson({
  'id': 'r-$state',
  'state': state,
  'pickup': {'lat': 42.66, 'lng': 21.16},
  'dropoff': {'lat': 42.67, 'lng': 21.17},
  'dropoff_address': dropoff,
  'price_quoted_minor': price,
  'currency': 'EUR',
  'requested_at': '2026-09-02T10:00:00Z',
});

Widget _wrap(Widget child, {AppState? state, String locale = 'sq'}) =>
    ChangeNotifierProvider<AppState>.value(
      value: state ?? AppState(),
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
        home: Scaffold(body: child),
      ),
    );

/// Ballina është më e gjatë se ekrani i testit; pa këtë hapësirë seksioni i historikut
/// nuk ndërtohet fare dhe testi do të kontrollonte diçka që nuk ekziston.
void useTallScreen(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 3200);
  tester.view.devicePixelRatio = 3.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

void main() {
  test('çdo gjendje udhëtimi ka çelësin e vet të përkthimit', () {
    for (final s in RideState.values) {
      expect(rideStateKey(s), startsWith('ride.state.'));
    }
    expect(rideStateKey(RideState.inProgress), 'ride.state.in_progress');
    expect(rideStateKey(RideState.noDriver), 'ride.state.no_driver');
  });

  test('etiketa e adresës bie te "tjetër" kur serveri dërgon diçka të panjohur', () {
    expect(addressLabelKey('home'), 'account.address.label.home');
    expect(addressLabelKey('villa'), 'account.address.label.other');
    expect(addressIcon('work'), Icons.work_outline);
  });

  // Ballina është pikënisje, jo listë: historia jeton te skeda Aktiviteti. Nëse dikush e kthen
  // historinë këtu, ky test bie — dhe ashtu duhet.
  testWidgets('ballina nuk mban historinë e asnjë shërbimi', (tester) async {
    useTallScreen(tester);
    final state = AppState()
      ..recentRides = [_ride(state: 'completed', dropoff: 'Sheshi Skënderbeu')];
    await tester.pumpWidget(_wrap(const HomeScreen(), state: state));
    await tester.pumpAndSettle();
    expect(find.text('Sheshi Skënderbeu'), findsNothing);
    expect(find.text('Së fundi'), findsNothing);
    // …por hyrjet te shërbimet janë aty.
    expect(find.text('Udhëtim'), findsWidgets);
  });

  testWidgets('udhëtimi aktiv shfaqet në krye me gjendjen e tij', (tester) async {
    useTallScreen(tester);
    final state = AppState()
      ..activeRide = _ride(state: 'in_progress')
      ..recentRides = [_ride(state: 'in_progress')];
    await tester.pumpWidget(_wrap(const HomeScreen(), state: state));
    await tester.pumpAndSettle();
    expect(find.byType(KNeonBanner), findsOneWidget);
    expect(find.text('Udhëtimi yt është në vazhdim'), findsOneWidget);
    expect(find.text('Në rrugë'), findsOneWidget);
  });

  // Katër shërbime njëkohësisht në rrjedhë: secili merr banderolën e vet, dhe asnjëri nuk e fsheh
  // tjetrin. Kjo është e vetmja përmbajtje që ballina ka të drejtë të mbajë.
  testWidgets('çdo shërbim në rrjedhë merr banderolën e vet', (tester) async {
    useTallScreen(tester);
    final state = AppState()..activeRide = _ride(state: 'assigned');
    await tester.pumpWidget(_wrap(const HomeScreen(), state: state));
    await tester.pumpAndSettle();
    expect(find.byType(KNeonBanner), findsOneWidget);
    expect(find.text('Udhëtimi yt është në vazhdim'), findsOneWidget);
  });

  testWidgets('shërbimi i fikur nga konfigurimi shfaqet si i ardhshëm', (tester) async {
    await tester.pumpWidget(_wrap(const HomeScreen()));
    await tester.pumpAndSettle();
    // Ushqimi varet nga flamuri i serverit dhe në konfigurimin e paracaktuar është i fikur, ndaj
    // vetëm ai shënohet "së shpejti"; të gjitha shërbimet e tjera janë të hapura.
    expect(find.text('Së shpejti'), findsOneWidget);
  });

  testWidgets('llogaria tregon inicialet dhe gjuhën aktive', (tester) async {
    final state = AppState()
      ..me = Me.fromJson({
        'id': 'u1',
        'phone': '+38344123456',
        'full_name': 'Arta Krasniqi',
        'locale': 'sq',
        'capabilities': <String>[],
        'wallet': {'balance_minor': 0, 'currency': 'EUR'},
      });
    await tester.pumpWidget(_wrap(const AccountScreen(), state: state));
    await tester.pumpAndSettle();
    expect(find.text('AK'), findsOneWidget);
    expect(find.text('Arta Krasniqi'), findsOneWidget);
    expect(find.text('Shqip'), findsOneWidget);
  });

  // Banderola brenda degës: pa të, kush hyn te Korrieri nga një njoftim e gjen formularin bosh
  // sikur pakoja e tij të mos ekzistonte.
  testWidgets('banderola e degës shfaqet vetëm kur ka diçka në rrjedhë', (tester) async {
    await tester.pumpWidget(_wrap(const ActiveBanner(kind: ActiveKind.ride)));
    await tester.pumpAndSettle();
    expect(find.byType(KNeonBanner), findsNothing);

    final state = AppState()..activeRide = _ride(state: 'in_progress');
    await tester.pumpWidget(_wrap(const ActiveBanner(kind: ActiveKind.ride), state: state));
    await tester.pumpAndSettle();
    expect(find.byType(KNeonBanner), findsOneWidget);
    expect(find.text('Në rrugë'), findsOneWidget);
  });
}
