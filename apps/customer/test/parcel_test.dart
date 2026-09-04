import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_customer/screens/parcels/new_parcel.dart';
import 'package:krejt_customer/screens/parcels/parcel_tracking.dart';
import 'package:krejt_customer/state/app_state.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

Widget _wrap(Widget child) => ChangeNotifierProvider(
  create: (_) => AppState(),
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
  testWidgets('dërgimi i pakos: tri madhësi, dy vende, çmimi i fikur pa vende', (tester) async {
    tester.view.physicalSize = const Size(1080, 2400);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.resetPhysicalSize);
    await tester.pumpWidget(_wrap(const NewParcelScreen()));
    await tester.pump();
    expect(find.byType(KMap), findsOneWidget);
    expect(find.text('E vogël'), findsOneWidget);
    expect(find.text('E mesme'), findsOneWidget);
    expect(find.text('E madhe'), findsOneWidget);
    expect(find.text('Ku e merr korrieri?'), findsOneWidget);
    expect(find.text('Ku dorëzohet?'), findsOneWidget);
    final button = tester.widget<KButton>(find.widgetWithText(KButton, 'Shih çmimin'));
    expect(button.onPressed, isNull);
    // Zgjedhja e madhësisë ndërron theksin pa thirrje rrjeti.
    await tester.tap(find.text('E madhe'));
    await tester.pump();
    expect(find.text('E madhe'), findsOneWidget);
  });

  test('çdo gjendje e pakos ka çelësin e vet të përkthimit', () {
    final keys = ParcelState.values.map(parcelStateKey).toSet();
    expect(keys.length, ParcelState.values.length);
    expect(parcelStateFrom('picked_up'), ParcelState.pickedUp);
    expect(parcelStateFrom('x'), ParcelState.requested);
  });
}
