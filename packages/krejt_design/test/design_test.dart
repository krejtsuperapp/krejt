import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_design/krejt_design.dart';

Widget _wrap(Widget child) => MaterialApp(
  theme: krejtTheme(),
  home: Scaffold(body: child),
);

void main() {
  testWidgets('shuma shfaqet e formatuar sipas gjuhës', (tester) async {
    await tester.pumpWidget(_wrap(const KMoney(1240)));
    expect(find.text('12,40 €'), findsOneWidget);

    await tester.pumpWidget(_wrap(const KMoney(1240, locale: 'en')));
    expect(find.text('€12.40'), findsOneWidget);
  });

  testWidgets('rreshti i faturës tregon etiketën dhe shumën', (tester) async {
    await tester.pumpWidget(_wrap(const KMoneyRow('Udhëtimi', 350)));
    expect(find.text('Udhëtimi'), findsOneWidget);
    expect(find.text('3,50 €'), findsOneWidget);
  });

  testWidgets('butoni në pritje nuk e thërret veprimin', (tester) async {
    var taps = 0;
    await tester.pumpWidget(_wrap(KButton(label: 'Vazhdo', busy: true, onPressed: () => taps++)));
    await tester.tap(find.byType(KButton));
    await tester.pump();
    expect(taps, 0);
  });

  testWidgets('butoni aktiv e thërret veprimin një herë', (tester) async {
    var taps = 0;
    await tester.pumpWidget(_wrap(KButton(label: 'Vazhdo', onPressed: () => taps++)));
    await tester.tap(find.byType(KButton));
    await tester.pump();
    expect(taps, 1);
  });

  testWidgets('zona e prekjes e butonit nuk bie nën 48 px', (tester) async {
    await tester.pumpWidget(_wrap(KButton(label: 'Vazhdo', onPressed: () {})));
    expect(tester.getSize(find.byType(KButton)).height, greaterThanOrEqualTo(K.minTap));
  });

  testWidgets('gjendja e gabimit ofron rikthim', (tester) async {
    var retries = 0;
    await tester.pumpWidget(
      _wrap(
        KError(message: 'Nuk ka lidhje', retryLabel: 'Provo përsëri', onRetry: () => retries++),
      ),
    );
    expect(find.text('Nuk ka lidhje'), findsOneWidget);
    await tester.tap(find.text('Provo përsëri'));
    await tester.pump();
    expect(retries, 1);
  });

  testWidgets('fleta e poshtme mban titullin dhe dorezën', (tester) async {
    await tester.pumpWidget(
      _wrap(
        Builder(
          builder: (context) => TextButton(
            onPressed: () => showKSheet<void>(
              context: context,
              title: 'Zgjidh kategorinë',
              child: const Text('Economy'),
            ),
            child: const Text('hap'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('hap'));
    await tester.pumpAndSettle();
    expect(find.text('Zgjidh kategorinë'), findsOneWidget);
    expect(find.byType(KSheetHandle), findsOneWidget);
  });

  testWidgets('fusha e kodit e dorëzon vlerën kur mbushen të gjitha shifrat', (tester) async {
    String? got;
    await tester.pumpWidget(_wrap(KOtpField(onCompleted: (v) => got = v)));
    await tester.enterText(find.byType(TextField).first, '123456');
    await tester.pumpAndSettle();
    expect(got, '123456');
  });
}
