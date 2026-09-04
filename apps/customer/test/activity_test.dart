import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_customer/screens/activity.dart';
import 'package:krejt_customer/screens/food/reorder.dart';

Ride _ride(String at, {String state = 'completed'}) => Ride.fromJson({
  'id': 'r1',
  'state': state,
  'pickup': {'lat': 42.66, 'lng': 21.16},
  'dropoff': {'lat': 42.67, 'lng': 21.17},
  'dropoff_address': 'Rr. Luan Haradinaj',
  'price_quoted_minor': 350,
  'currency': 'EUR',
  'requested_at': at,
});

Order _order(String at, {String state = 'delivered'}) => Order.fromJson({
  'id': 'o1',
  'code': 'K1',
  'merchant_id': 'm1',
  'merchant_name': 'Te Syla',
  'state': state,
  'fulfillment': 'delivery',
  'total_minor': 720,
  'currency': 'EUR',
  'created_at': at,
});

Parcel _parcel(String at) => Parcel.fromJson({
  'id': 'p1',
  'code': 'K2',
  'state': 'delivered',
  'size': 's',
  'pickup': {'lat': 42.66, 'lng': 21.16},
  'dropoff': {'lat': 42.67, 'lng': 21.17},
  'recipient_name': 'Arta',
  'recipient_phone': '+38344000000',
  'price_minor': 200,
  'currency': 'EUR',
  'created_at': at,
});

ServiceRequest _service(String at) => ServiceRequest.fromJson({
  'id': 's1',
  'code': 'K3',
  'category_id': 'c1',
  'state': 'completed',
  'title': 'Rrjedh rubineti',
  'description': 'Kuzhina',
  'address_line1': 'Dardania',
  'address': {'lat': 42.65, 'lng': 21.15},
  'currency': 'EUR',
  'created_at': at,
});

void main() {
  _reorderTests();
  test('të katër shërbimet bashkohen në një listë, nga më e reja te më e vjetra', () {
    final out = mergeActivity(
      rides: [_ride('2026-09-01T10:00:00Z')],
      orders: [_order('2026-09-03T10:00:00Z')],
      parcels: [_parcel('2026-09-02T10:00:00Z')],
      services: [_service('2026-08-30T10:00:00Z')],
    );
    expect(out.map((e) => e.kind).toList(), [
      ActivityKind.order,
      ActivityKind.parcel,
      ActivityKind.ride,
      ActivityKind.service,
    ]);
    expect(out.first.title, 'Te Syla');
    expect(out.first.amountMinor, 720);
  });

  test('lista bosh mbetet bosh; një burim i vetëm mjafton', () {
    expect(
      mergeActivity(rides: const [], orders: const [], parcels: const [], services: const []),
      isEmpty,
    );
    final one = mergeActivity(
      rides: [_ride('2026-09-01T10:00:00Z')],
      orders: const [],
      parcels: const [],
      services: const [],
    );
    expect(one.single.title, 'Rr. Luan Haradinaj');
  });

  test('anulimi shënohet që çmimi të dalë i hequr me vijë', () {
    final out = mergeActivity(
      rides: [_ride('2026-09-01T10:00:00Z', state: 'cancelled')],
      orders: [_order('2026-09-02T10:00:00Z', state: 'rejected')],
      parcels: const [],
      services: const [],
    );
    expect(out.every((e) => e.cancelled), isTrue);
  });

  test('çmimi mungon te kërkesa pa ofertë të pranuar', () {
    final out = mergeActivity(
      rides: const [],
      orders: const [],
      parcels: const [],
      services: [_service('2026-09-01T10:00:00Z')],
    );
    expect(out.single.amountMinor, isNull);
  });

  test('çdo lloj ka etiketën dhe ikonën e vet', () {
    final keys = ActivityKind.values.map(activityLabelKey).toSet();
    expect(keys.length, ActivityKind.values.length);
    expect(keys.every((k) => k.startsWith('home.services.')), isTrue);
    expect(ActivityKind.values.map(activityIcon).toSet().length, ActivityKind.values.length);
  });
}

/// Riporositja: çmimet dhe disponueshmëria vijnë nga menuja e sotme, kurrë nga porosia e ruajtur.
void _reorderTests() {
  Product prod(String id, {bool available = true, List<String> options = const []}) =>
      Product.fromJson({
        'id': id,
        'merchant_id': 'm1',
        'name': 'Pjata $id',
        'price_minor': 500,
        'currency': 'EUR',
        'available': available,
        'modifiers': [
          if (options.isNotEmpty)
            {
              'id': 'g1',
              'name': 'Shtesa',
              'options': [
                for (final o in options) {'id': o, 'name': o, 'price_delta_minor': 0},
              ],
            },
        ],
      });

  Order orderWith(List<Map<String, dynamic>> items) => Order.fromJson({
    'id': 'o1',
    'code': 'K1',
    'merchant_id': 'm1',
    'merchant_name': 'Te Syla',
    'state': 'delivered',
    'fulfillment': 'delivery',
    'total_minor': 1000,
    'currency': 'EUR',
    'created_at': '2026-09-03T10:00:00Z',
    'items': items,
  });

  Menu menuOf(List<Product> products) =>
      Menu(merchantId: 'm1', categories: const [], products: products);

  test('artikujt e hequr nga menuja nuk shtohen dhe emërtohen', () {
    final order = orderWith([
      {'id': 'i1', 'product_id': 'p1', 'name': 'Qebapa', 'quantity': 2},
      {'id': 'i2', 'product_id': 'p9', 'name': 'Sallatë e vjetër', 'quantity': 1},
    ]);
    final out = rebuildCart(order, menuOf([prod('p1')]));
    expect(out.lines.length, 1);
    expect(out.lines.single.quantity, 2);
    expect(out.missing, ['Sallatë e vjetër']);
  });

  test('artikulli i palejuar sot trajtohet si i munguar', () {
    final order = orderWith([
      {'id': 'i1', 'product_id': 'p1', 'name': 'Qebapa', 'quantity': 1},
    ]);
    final out = rebuildCart(order, menuOf([prod('p1', available: false)]));
    expect(out.isEmpty, isTrue);
    expect(out.missing, ['Qebapa']);
  });

  test('opsionet që nuk ekzistojnë më bien, produkti mbetet', () {
    final order = orderWith([
      {
        'id': 'i1',
        'product_id': 'p1',
        'name': 'Qebapa',
        'quantity': 1,
        'option_ids': ['kajmak', 'i_hequr'],
      },
    ]);
    final out = rebuildCart(
      order,
      menuOf([
        prod('p1', options: ['kajmak']),
      ]),
    );
    expect(out.lines.single.optionIds, ['kajmak']);
    expect(out.missing, isEmpty);
  });
}
