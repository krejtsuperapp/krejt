import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:krejt_customer/screens/food/cart_bar.dart';
import 'package:krejt_customer/state/app_state.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_customer/screens/food/discover.dart';
import 'package:krejt_customer/screens/food/order_tracking.dart';
import 'package:krejt_customer/state/cart_state.dart';

Product _product(String id, {String name = 'Pite', int price = 250}) => Product.fromJson({
  'id': id,
  'merchant_id': 'm1',
  'name': name,
  'price_minor': price,
  'currency': 'EUR',
});

Merchant _merchant(String id, {bool open = true, int minOrder = 500}) => Merchant.fromJson({
  'id': id,
  'name': 'Vendi $id',
  'slug': 'vendi-$id',
  'type': 'restaurant',
  'location': {'lat': 42.66, 'lng': 21.16},
  'status': 'active',
  'min_order_minor': minOrder,
  'open_now': open,
  'accepting_orders': open,
});

void main() {
  _cartBarTests();
  group('shporta', () {
    test('zërat e njëjtë bashkohen në një rresht të vetëm', () {
      final cart = CartState()..startAt(_merchant('m1'));
      cart.add(CartLine(product: _product('p1'), optionIds: const [], quantity: 1));
      cart.add(CartLine(product: _product('p1'), optionIds: const [], quantity: 2));
      expect(cart.lines.length, 1);
      expect(cart.lines.first.quantity, 3);
      expect(cart.itemCount, 3);
    });

    test('opsione të ndryshme mbeten rreshta të veçantë', () {
      final cart = CartState()..startAt(_merchant('m1'));
      cart.add(CartLine(product: _product('p1'), optionIds: const ['a'], quantity: 1));
      cart.add(CartLine(product: _product('p1'), optionIds: const ['b'], quantity: 1));
      expect(cart.lines.length, 2);
    });

    test('renditja e opsioneve nuk krijon rresht të ri', () {
      final cart = CartState()..startAt(_merchant('m1'));
      cart.add(CartLine(product: _product('p1'), optionIds: const ['a', 'b'], quantity: 1));
      cart.add(CartLine(product: _product('p1'), optionIds: const ['b', 'a'], quantity: 1));
      expect(cart.lines.length, 1);
      expect(cart.lines.first.quantity, 2);
    });

    test('sasia zero e heq zërin dhe shporta e zbrazët humb vendin', () {
      final cart = CartState()..startAt(_merchant('m1'));
      cart.add(CartLine(product: _product('p1'), optionIds: const [], quantity: 2));
      cart.setQuantity(0, 0);
      expect(cart.isEmpty, isTrue);
      expect(cart.merchant, isNull);
    });

    test('kalimi te një vend tjetër e zbraz shportën', () {
      final cart = CartState()..startAt(_merchant('m1'));
      cart.add(CartLine(product: _product('p1'), optionIds: const [], quantity: 1));
      cart.startAt(_merchant('m2'));
      expect(cart.isEmpty, isTrue);
      expect(cart.belongsTo('m2'), isTrue);
    });
  });

  group('oferta e porosisë', () {
    OrderQuote quote({int items = 400, int minimum = 500, bool open = true}) =>
        OrderQuote.fromJson({
          'items_total_minor': items,
          'delivery_fee_minor': 100,
          'total_minor': items + 100,
          'min_order_minor': minimum,
          'currency': 'EUR',
          'open_now': open,
        });

    test('nën minimumin tregon sa mungon dhe nuk lejon porosinë', () {
      final q = quote();
      expect(q.missingForMinimum, 100);
      expect(q.canCheckout, isFalse);
    });

    test('mbi minimumin lejon porosinë', () {
      final q = quote(items: 800);
      expect(q.missingForMinimum, 0);
      expect(q.canCheckout, isTrue);
    });

    test('vendi i mbyllur nuk lejon porosi edhe kur shuma mjafton', () {
      expect(quote(items: 800, open: false).canCheckout, isFalse);
    });
  });

  group('vendi dhe porosia', () {
    test('vetëm vendi i hapur dhe që pranon lejon porosi', () {
      expect(_merchant('m1').canOrder, isTrue);
      expect(_merchant('m2', open: false).canOrder, isFalse);
    });

    test('çdo lloj vendi ka çelësin e vet, i panjohuri bie te restoranti', () {
      for (final type in merchantTypes) {
        expect(merchantTypeKey(type), 'food.type.$type');
      }
      expect(merchantTypeKey('kiosk'), 'food.type.restaurant');
    });

    test('çdo gjendje porosie ka tekstin e vet', () {
      for (final s in OrderState.values) {
        expect(orderStateKey(s), startsWith('order.state.'));
      }
      expect(orderStateKey(OrderState.pickedUp), 'order.state.picked_up');
    });

    test('anulimi lejohet vetëm para se kuzhina të nisë', () {
      Order order(String state) => Order.fromJson({
        'id': 'o1',
        'code': 'K7F3QA',
        'state': state,
        'currency': 'EUR',
        'created_at': '2026-09-02T10:00:00Z',
      });
      expect(order('pending_merchant').canCancel, isTrue);
      expect(order('accepted').canCancel, isTrue);
      expect(order('preparing').canCancel, isFalse);
      expect(order('picked_up').canCancel, isFalse);
      expect(order('delivered').isActive, isFalse);
    });
  });
}

/// Shiriti i shportës: shfaqet vetëm kur ka çka të tregojë, dhe mban emrin e lokalit — pa të,
/// «3 artikuj» nuk i thotë përdoruesit se ku e la porosinë.
void _cartBarTests() {
  Widget wrap(CartState cart) => MultiProvider(
    providers: [
      ChangeNotifierProvider<CartState>.value(value: cart),
      ChangeNotifierProvider<AppState>(create: (_) => AppState()),
    ],
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
      home: const Scaffold(body: Column(children: [Spacer(), CartBar()])),
    ),
  );

  testWidgets('shporta bosh nuk zë vend fare', (tester) async {
    await tester.pumpWidget(wrap(CartState()));
    await tester.pumpAndSettle();
    expect(find.text('Shporta'), findsNothing);
  });

  testWidgets('shporta me artikuj tregon sasinë dhe lokalin', (tester) async {
    final cart = CartState()..startAt(_merchant('m1'));
    cart.add(CartLine(product: _product('p1'), optionIds: const [], quantity: 2));
    await tester.pumpWidget(wrap(cart));
    await tester.pumpAndSettle();
    expect(find.text('Shporta'), findsOneWidget);
    expect(find.text('2'), findsOneWidget);
    expect(find.text('Vendi m1'), findsOneWidget);
  });
}
