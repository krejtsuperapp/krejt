import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_driver/screens/bank.dart';
import 'package:krejt_driver/screens/documents.dart';
import 'package:krejt_driver/screens/courier.dart';
import 'package:krejt_driver/screens/offer_card.dart';
import 'package:krejt_driver/screens/sign_in.dart';
import 'package:krejt_driver/screens/work.dart';
import 'package:krejt_driver/state/app_state.dart';
import 'package:krejt_driver/state/work_state.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

RideOffer _offer({int seconds = 15, int earnings = 480}) => RideOffer.fromJson({
  'id': 'o1',
  'ride_id': 'r1',
  'expires_at': DateTime.now().add(Duration(seconds: seconds)).toIso8601String(),
  'distance_m': 1200,
  'eta_s': 240,
  'category': 'economy',
  'pickup': {'lat': 42.66, 'lng': 21.16},
  'pickup_address': 'Rruga A 1',
  'dropoff': {'lat': 42.67, 'lng': 21.17},
  'dropoff_address': 'Rruga B 2',
  'ride_distance_m': 5400,
  'ride_duration_s': 900,
  'price_minor': 600,
  'earnings_minor': earnings,
  'currency': 'EUR',
  'payment_method': 'cash',
});

Widget _wrap(Widget child, {AppState? app, WorkState? work, String locale = 'sq'}) {
  final appState = app ?? AppState();
  return MultiProvider(
    providers: [
      ChangeNotifierProvider<AppState>.value(value: appState),
      ChangeNotifierProvider<WorkState>.value(value: work ?? WorkState(api: appState.api)),
    ],
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
}

CourierOffer _delivery({int seconds = 25, int earnings = 220}) => CourierOffer.fromJson({
  'id': 'd1',
  'order_id': 'o1',
  'code': 'K7F3QA',
  'expires_at': DateTime.now().add(Duration(seconds: seconds)).toIso8601String(),
  'distance_m': 900,
  'eta_s': 180,
  'merchant_name': 'Te Ura',
  'merchant_address': 'Rruga C 3',
  'merchant_location': {'lat': 42.66, 'lng': 21.16},
  'dropoff_address': 'Rruga D 4',
  'dropoff': {'lat': 42.67, 'lng': 21.17},
  'earnings_minor': earnings,
  'currency': 'EUR',
  'payment_method': 'cash',
  'total_minor': 1450,
});

void main() {
  group('IBAN', () {
    test('IBAN-i i saktë pranohet, me ose pa hapësira', () {
      expect(isValidIban('XK051212012345678906'), isTrue);
      expect(isValidIban('XK05 1212 0123 4567 8906'), isTrue);
      expect(isValidIban('xk05 1212 0123 4567 8906'), isTrue);
      expect(isValidIban('DE89370400440532013000'), isTrue);
    });

    test('një shifër e ndryshuar e prish kontrollin mod-97', () {
      expect(isValidIban('XK051212012345678907'), isFalse);
      expect(isValidIban('DE89370400440532013001'), isFalse);
    });

    test('forma e gabuar refuzohet para se të llogaritet gjë', () {
      expect(isValidIban(''), isFalse);
      expect(isValidIban('XK05'), isFalse);
      expect(isValidIban('1205 1212 0123 4567 8906'), isFalse);
      expect(isValidIban('XK-5 1212 0123 4567 8906'), isFalse);
    });
  });

  test('çdo lloj dokumenti dhe çdo status ka çelësin e vet', () {
    for (final type in documentTypes) {
      expect(documentTypeKey(type), 'driver.docs.type.$type');
    }
    for (final s in DocumentStatus.values) {
      expect(documentStatusKey(s), 'driver.docs.status.${s.name}');
    }
    expect(documentTone(DocumentStatus.approved), KTone.ok);
    expect(documentTone(DocumentStatus.rejected), KTone.danger);
  });

  test('hapat e udhëtimit kanë secili tekstin e vet', () {
    expect(driverRideStateKey(RideState.assigned), 'driver.ride.to_pickup');
    expect(driverRideStateKey(RideState.arrived), 'driver.ride.waiting');
    expect(driverRideStateKey(RideState.inProgress), 'driver.ride.driving');
  });

  test('oferta e skaduar nuk zgjidhet si e para', () {
    final work = WorkState(api: AppState().api);
    work.offers = [
      RideOffer.fromJson({
        'id': 'expired',
        'ride_id': 'r0',
        'expires_at': DateTime.now().subtract(const Duration(seconds: 5)).toIso8601String(),
        'pickup': {'lat': 0, 'lng': 0},
        'dropoff': {'lat': 0, 'lng': 0},
        'earnings_minor': 100,
      }),
      _offer(),
    ];
    expect(work.topOffer?.id, 'o1');
  });

  test('pa oferta të vlefshme nuk ka çfarë të pranohet', () {
    final work = WorkState(api: AppState().api);
    expect(work.topOffer, isNull);
    expect(work.online, isFalse);
  });

  testWidgets('hyrja e shoferit kërkon numrin e telefonit', (tester) async {
    await tester.pumpWidget(_wrap(const SignInScreen()));
    await tester.pumpAndSettle();
    expect(find.text('Numri i telefonit'), findsOneWidget);
  });

  testWidgets('pa profil të aprovuar nuk shfaqet butoni i hyrjes në punë', (tester) async {
    await tester.pumpWidget(_wrap(const WorkScreen()));
    await tester.pump();
    expect(find.text('Llogaria është në shqyrtim'), findsOneWidget);
    expect(find.text('Fillo punën'), findsNothing);
  });

  testWidgets('pa aprovim nuk përsëritet arsyeja poshtë kartës', (tester) async {
    await tester.pumpWidget(_wrap(const WorkScreen()));
    await tester.pump();
    expect(find.text('Jashtë pune'), findsOneWidget);
    expect(find.text('Fillo punën për të marrë kërkesa.'), findsNothing);
  });

  testWidgets('kërkesa tregon fitimin, numërimin dhe të dy përgjigjet', (tester) async {
    await tester.pumpWidget(_wrap(OfferCard(offer: _offer(earnings: 480))));
    await tester.pump();
    expect(find.text('Kërkesë e re'), findsOneWidget);
    expect(find.text('4,80 €'), findsOneWidget);
    expect(find.text('Prano'), findsOneWidget);
    expect(find.text('Refuzo'), findsOneWidget);
    expect(find.text('Rruga A 1'), findsOneWidget);
  });

  test('puna aktive i ndal ofertat e reja të të dyja llojeve', () {
    final work = WorkState(api: AppState().api);
    expect(work.isBusyWithWork, isFalse);
    work.deliveryOffers = [_delivery()];
    expect(work.topDeliveryOffer?.id, 'd1');
    work.deliveryOffers = [_delivery(seconds: 0)];
    expect(work.topDeliveryOffer, isNull);
  });

  testWidgets('dorëzimi tregon të dy adresat dhe fitimin', (tester) async {
    await tester.pumpWidget(_wrap(CourierOfferCard(offer: _delivery(earnings: 220))));
    await tester.pump();
    expect(find.text('Dorëzim i ri'), findsOneWidget);
    expect(find.text('2,20 €'), findsOneWidget);
    expect(find.textContaining('Te Ura'), findsOneWidget);
    expect(find.text('Rruga D 4'), findsOneWidget);
  });

  testWidgets('gjendja e dorëzimit ndan para dhe pas marrjes', (tester) async {
    expect(courierOrderStateKey(OrderState.pickedUp), 'order.state.picked_up');
    expect(courierOrderStateKey(OrderState.courierAssigned), 'order.state.ready');
  });

  testWidgets('kërkesa e skaduar nuk pranohet dot', (tester) async {
    await tester.pumpWidget(_wrap(OfferCard(offer: _offer(seconds: 0))));
    await tester.pump();
    final accept = tester.widget<KButton>(
      find.ancestor(of: find.text('Prano'), matching: find.byType(KButton)),
    );
    expect(accept.onPressed, isNull);
  });
}
