import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_customer/screens/ride/destination.dart';
import 'package:krejt_customer/screens/ride/quote.dart';
import 'package:krejt_customer/screens/ride/review.dart';
import 'package:krejt_customer/services/location.dart';
import 'package:krejt_customer/state/app_state.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

Ride _completedRide() => Ride.fromJson({
  'id': 'r1',
  'state': 'completed',
  'pickup': {'lat': 42.66, 'lng': 21.16},
  'dropoff': {'lat': 42.67, 'lng': 21.17},
  'dropoff_address': 'Rruga B 12',
  'price_quoted_minor': 350,
  'price_final_minor': 380,
  'currency': 'EUR',
  'requested_at': '2026-09-02T10:00:00Z',
  'completed_at': '2026-09-02T10:20:00Z',
});

Widget _wrap(Widget child, {AppState? state}) => ChangeNotifierProvider<AppState>.value(
  value: state ?? AppState(),
  child: MaterialApp(
    theme: krejtTheme(),
    locale: const Locale('sq'),
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
  test('çdo kategori udhëtimi ka çelësin e vet të përkthimit', () {
    for (final c in RideCategory.values) {
      expect(rideCategoryKey(c), 'ride.category.${c.name}');
    }
  });

  test('çdo arsye e mungesës së vendndodhjes ka mesazhin e vet', () {
    for (final p in LocationProblem.values) {
      expect(locationProblemKey(p), startsWith('location.'));
    }
    expect(locationProblemKey(LocationProblem.deniedForever), 'location.denied_forever');
  });

  test('oferta e çmimit njeh skadimin dhe kërkesën e lartë', () {
    final live = RideQuote.fromJson({
      'id': 'q1',
      'category': 'comfort',
      'price_minor': 420,
      'surge_bp': 13000,
      'expires_at': DateTime.now().add(const Duration(minutes: 2)).toIso8601String(),
    });
    expect(live.expired, isFalse);
    expect(live.surging, isTrue);

    final stale = RideQuote.fromJson({
      'id': 'q2',
      'category': 'economy',
      'price_minor': 300,
      'surge_bp': 10000,
      'expires_at': DateTime.now().subtract(const Duration(seconds: 1)).toIso8601String(),
    });
    expect(stale.expired, isTrue);
    expect(stale.surging, isFalse);
  });

  test('biseda mbyllet 24 orë pas përfundimit', () {
    Ride ride(String completedAt) => Ride.fromJson({
      'id': 'r1',
      'state': 'completed',
      'driver_id': 'd1',
      'pickup': {'lat': 0, 'lng': 0},
      'dropoff': {'lat': 0, 'lng': 0},
      'price_quoted_minor': 100,
      'requested_at': '2026-09-01T10:00:00Z',
      'completed_at': completedAt,
    });

    final justNow = DateTime.now().subtract(const Duration(hours: 2));
    final longAgo = DateTime.now().subtract(const Duration(hours: 25));
    expect(ride(justNow.toIso8601String()).chatOpen, isTrue);
    expect(ride(longAgo.toIso8601String()).chatOpen, isFalse);
  });

  testWidgets('vlerësimi nis me pesë yje dhe mund të kalohet', (tester) async {
    await tester.pumpWidget(_wrap(ReviewScreen(ride: _completedRide())));
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.star_rounded), findsNWidgets(5));
    expect(find.text('Më vonë'), findsOneWidget);
    expect(find.text('Rruga B 12'), findsOneWidget);
    expect(find.text('3,80 €'), findsOneWidget);
  });

  testWidgets('prekja e yllit të dytë lë vetëm dy yje të mbushur', (tester) async {
    await tester.pumpWidget(_wrap(ReviewScreen(ride: _completedRide())));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.star_rounded).at(1));
    await tester.pump();
    expect(find.byIcon(Icons.star_rounded), findsNWidgets(2));
    expect(find.byIcon(Icons.star_outline_rounded), findsNWidgets(3));
  });

  testWidgets('etiketat e vlerësimit janë të përkthyera dhe të zgjedhshme', (tester) async {
    await tester.pumpWidget(_wrap(ReviewScreen(ride: _completedRide())));
    await tester.pumpAndSettle();
    for (final tag in reviewTags) {
      expect(find.byType(FilterChip), findsWidgets, reason: tag);
    }
    expect(find.text('Makinë e pastër'), findsOneWidget);
    await tester.tap(find.text('Makinë e pastër'));
    await tester.pump();
    final chip = tester.widget<FilterChip>(
      find.ancestor(of: find.text('Makinë e pastër'), matching: find.byType(FilterChip)),
    );
    expect(chip.selected, isTrue);
  });

  testWidgets('zgjedhja e destinacionit: hartë, dy fusha dhe Vazhdo i fikur pa destinacion', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1080, 2280);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.resetPhysicalSize);
    await tester.pumpWidget(_wrap(const DestinationScreen()));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.byType(KMap), findsOneWidget);
    expect(find.text('Ku po shkon?'), findsOneWidget);
    final button = tester.widget<KButton>(find.byType(KButton));
    expect(button.onPressed, isNull);
    // Kërkimi nis vetëm pas dy shkronjave dhe pas pauzës — asnjë thirrje për një shkronjë.
    await tester.enterText(find.byType(TextField), 'S');
    await tester.pump();
    expect(find.byType(KSkeleton), findsNothing);
    await tester.pump(const Duration(seconds: 1));
  });
}
