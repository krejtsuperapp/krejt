import 'ride.dart';

/// Modelet e ushqimit, marketit dhe dorëzimit (§19, §21, §26). Çmimet vijnë vetëm nga serveri;
/// shporta në pajisje mban sasi dhe zgjedhje, kurrë shuma.

class Merchant {
  Merchant({
    required this.id,
    required this.type,
    required this.name,
    required this.slug,
    this.description,
    required this.addressLine1,
    required this.city,
    required this.location,
    required this.status,
    required this.cuisines,
    required this.fulfillmentMode,
    required this.minOrderMinor,
    required this.deliveryFeeMinor,
    required this.prepTimeMin,
    this.rating,
    required this.ratingCount,
    required this.acceptingOrders,
    required this.openNow,
    required this.distanceM,
  });

  final String id;
  final String type;
  final String name;
  final String slug;
  final String? description;
  final String addressLine1;
  final String city;
  final LatLng location;
  final String status;
  final List<String> cuisines;
  final String fulfillmentMode;
  final int minOrderMinor;
  final int deliveryFeeMinor;
  final int prepTimeMin;
  final double? rating;
  final int ratingCount;
  final bool acceptingOrders;
  final bool openNow;
  final int distanceM;

  /// I hapur dhe duke pranuar: vetëm atëherë ka kuptim të hapësh menunë për porosi.
  bool get canOrder => openNow && acceptingOrders && status == 'active';

  factory Merchant.fromJson(Map<String, dynamic> j) => Merchant(
    id: j['id'].toString(),
    type: (j['type'] ?? 'restaurant').toString(),
    name: (j['name'] ?? '').toString(),
    slug: (j['slug'] ?? '').toString(),
    description: j['description']?.toString(),
    addressLine1: (j['address_line1'] ?? '').toString(),
    city: (j['city'] ?? '').toString(),
    location: LatLng.fromJson(
      Map<String, dynamic>.from((j['location'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    status: (j['status'] ?? 'active').toString(),
    cuisines: ((j['cuisines'] as List?) ?? const []).map((e) => e.toString()).toList(),
    fulfillmentMode: (j['fulfillment_mode'] ?? 'courier').toString(),
    minOrderMinor: (j['min_order_minor'] as num?)?.toInt() ?? 0,
    deliveryFeeMinor: (j['delivery_fee_minor'] as num?)?.toInt() ?? 0,
    prepTimeMin: (j['prep_time_min'] as num?)?.toInt() ?? 0,
    rating: (j['rating'] as num?)?.toDouble(),
    ratingCount: (j['rating_count'] as num?)?.toInt() ?? 0,
    acceptingOrders: j['accepting_orders'] != false,
    openNow: j['open_now'] == true,
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
  );
}

class ModifierOption {
  ModifierOption({
    required this.id,
    required this.name,
    required this.priceDeltaMinor,
    required this.available,
  });

  final String id;
  final String name;
  final int priceDeltaMinor;
  final bool available;

  factory ModifierOption.fromJson(Map<String, dynamic> j) => ModifierOption(
    id: j['id'].toString(),
    name: (j['name'] ?? '').toString(),
    priceDeltaMinor: (j['price_delta_minor'] as num?)?.toInt() ?? 0,
    available: j['available'] != false,
  );
}

class ModifierGroup {
  ModifierGroup({
    required this.id,
    required this.name,
    required this.minSelect,
    required this.maxSelect,
    required this.options,
  });

  final String id;
  final String name;
  final int minSelect;
  final int maxSelect;
  final List<ModifierOption> options;

  bool get required => minSelect > 0;
  bool get single => maxSelect == 1;

  factory ModifierGroup.fromJson(Map<String, dynamic> j) => ModifierGroup(
    id: j['id'].toString(),
    name: (j['name'] ?? '').toString(),
    minSelect: (j['min_select'] as num?)?.toInt() ?? 0,
    maxSelect: (j['max_select'] as num?)?.toInt() ?? 1,
    options: ((j['options'] as List?) ?? const [])
        .map((e) => ModifierOption.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
  );
}

class Product {
  Product({
    required this.id,
    required this.merchantId,
    this.categoryId,
    required this.name,
    this.description,
    required this.priceMinor,
    required this.currency,
    required this.available,
    required this.unit,
    required this.modifiers,
  });

  final String id;
  final String merchantId;
  final String? categoryId;
  final String name;
  final String? description;
  final int priceMinor;
  final String currency;
  final bool available;
  final String unit;
  final List<ModifierGroup> modifiers;

  factory Product.fromJson(Map<String, dynamic> j) => Product(
    id: j['id'].toString(),
    merchantId: (j['merchant_id'] ?? '').toString(),
    categoryId: j['category_id']?.toString(),
    name: (j['name'] ?? '').toString(),
    description: j['description']?.toString(),
    priceMinor: (j['price_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    available: j['available'] != false,
    unit: (j['unit'] ?? 'piece').toString(),
    modifiers: ((j['modifiers'] as List?) ?? const [])
        .map((e) => ModifierGroup.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
  );
}

class MenuCategory {
  MenuCategory({required this.id, required this.name, required this.sort});

  final String id;
  final String name;
  final int sort;

  factory MenuCategory.fromJson(Map<String, dynamic> j) => MenuCategory(
    id: j['id'].toString(),
    name: (j['name'] ?? '').toString(),
    sort: (j['sort'] as num?)?.toInt() ?? 0,
  );
}

class Menu {
  Menu({required this.merchantId, required this.categories, required this.products});

  final String merchantId;
  final List<MenuCategory> categories;
  final List<Product> products;

  List<Product> inCategory(String categoryId) =>
      products.where((p) => p.categoryId == categoryId).toList();

  factory Menu.fromJson(Map<String, dynamic> j) => Menu(
    merchantId: (j['merchant_id'] ?? '').toString(),
    categories: ((j['categories'] as List?) ?? const [])
        .map((e) => MenuCategory.fromJson(Map<String, dynamic>.from(e as Map)))
        .where((c) => c.name.isNotEmpty)
        .toList(),
    products: ((j['products'] as List?) ?? const [])
        .map((e) => Product.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
  );
}

/// Një zë i shportës: produkt, opsionet e zgjedhura dhe sasia. Çmimin e llogarit serveri.
class CartLine {
  CartLine({required this.product, required this.optionIds, required this.quantity});

  final Product product;
  final List<String> optionIds;
  final int quantity;

  CartLine copyWith({int? quantity}) =>
      CartLine(product: product, optionIds: optionIds, quantity: quantity ?? this.quantity);

  /// Dy zëra bashkohen vetëm kur janë i njëjti produkt me të njëjtat opsione.
  bool sameAs(CartLine other) {
    if (product.id != other.product.id) return false;
    if (optionIds.length != other.optionIds.length) return false;
    final a = [...optionIds]..sort();
    final b = [...other.optionIds]..sort();
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }

  Map<String, dynamic> toJson() => {
    'product_id': product.id,
    if (optionIds.isNotEmpty) 'option_ids': optionIds,
    'quantity': quantity,
  };
}

class OrderQuoteLine {
  OrderQuoteLine({
    required this.name,
    required this.options,
    required this.unitMinor,
    required this.quantity,
    required this.totalMinor,
  });

  final String name;
  final List<String> options;
  final int unitMinor;
  final int quantity;
  final int totalMinor;

  factory OrderQuoteLine.fromJson(Map<String, dynamic> j) => OrderQuoteLine(
    name: (j['name'] ?? '').toString(),
    options: ((j['options'] as List?) ?? const []).map((e) => e.toString()).toList(),
    unitMinor: (j['unit_minor'] as num?)?.toInt() ?? 0,
    quantity: (j['quantity'] as num?)?.toInt() ?? 1,
    totalMinor: (j['total_minor'] as num?)?.toInt() ?? 0,
  );
}

class OrderQuote {
  OrderQuote({
    required this.items,
    required this.itemsTotalMinor,
    required this.deliveryFeeMinor,
    required this.totalMinor,
    required this.minOrderMinor,
    required this.currency,
    required this.prepTimeMin,
    required this.openNow,
  });

  final List<OrderQuoteLine> items;
  final int itemsTotalMinor;
  final int deliveryFeeMinor;
  final int totalMinor;
  final int minOrderMinor;
  final String currency;
  final int prepTimeMin;
  final bool openNow;

  /// Sa mungon deri te porosia minimale, ose zero kur është arritur.
  int get missingForMinimum {
    final missing = minOrderMinor - itemsTotalMinor;
    return missing > 0 ? missing : 0;
  }

  bool get canCheckout => openNow && missingForMinimum == 0;

  factory OrderQuote.fromJson(Map<String, dynamic> j) => OrderQuote(
    items: ((j['items'] as List?) ?? const [])
        .map((e) => OrderQuoteLine.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
    itemsTotalMinor: (j['items_total_minor'] as num?)?.toInt() ?? 0,
    deliveryFeeMinor: (j['delivery_fee_minor'] as num?)?.toInt() ?? 0,
    totalMinor: (j['total_minor'] as num?)?.toInt() ?? 0,
    minOrderMinor: (j['min_order_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    prepTimeMin: (j['prep_time_min'] as num?)?.toInt() ?? 0,
    openNow: j['open_now'] == true,
  );
}

enum OrderState {
  pendingMerchant,
  accepted,
  preparing,
  ready,
  courierAssigned,
  pickedUp,
  delivered,
  cancelled,
  rejected,
}

OrderState orderStateFrom(String s) {
  switch (s) {
    case 'pending_merchant':
      return OrderState.pendingMerchant;
    case 'accepted':
      return OrderState.accepted;
    case 'preparing':
      return OrderState.preparing;
    case 'ready':
      return OrderState.ready;
    case 'courier_assigned':
      return OrderState.courierAssigned;
    case 'picked_up':
      return OrderState.pickedUp;
    case 'delivered':
      return OrderState.delivered;
    case 'rejected':
      return OrderState.rejected;
    default:
      return OrderState.cancelled;
  }
}

class OrderItem {
  OrderItem({
    required this.id,
    required this.name,
    required this.options,
    required this.unitMinor,
    required this.quantity,
    required this.totalMinor,
  });

  final String id;
  final String name;
  final List<String> options;
  final int unitMinor;
  final int quantity;
  final int totalMinor;

  factory OrderItem.fromJson(Map<String, dynamic> j) => OrderItem(
    id: j['id'].toString(),
    name: (j['name'] ?? '').toString(),
    options: ((j['options'] as List?) ?? const []).map((e) => e.toString()).toList(),
    unitMinor: (j['unit_minor'] as num?)?.toInt() ?? 0,
    quantity: (j['quantity'] as num?)?.toInt() ?? 1,
    totalMinor: (j['total_minor'] as num?)?.toInt() ?? 0,
  );
}

class Order {
  Order({
    required this.id,
    required this.code,
    required this.merchantId,
    required this.merchantName,
    this.courierId,
    required this.state,
    required this.fulfillment,
    required this.paymentMethod,
    required this.paymentStatus,
    required this.itemsTotalMinor,
    required this.deliveryFeeMinor,
    required this.discountMinor,
    required this.totalMinor,
    required this.currency,
    this.addressText,
    this.note,
    required this.prepTimeMin,
    this.readyAtEstimate,
    this.cancellationReason,
    required this.createdAt,
    this.deliveredAt,
    required this.items,
  });

  final String id;

  /// Kodi 6-shkronjor që korrieri i thotë merchant-it kur merr porosinë.
  final String code;
  final String merchantId;
  final String merchantName;
  final String? courierId;
  final OrderState state;
  final String fulfillment;
  final String paymentMethod;
  final String paymentStatus;
  final int itemsTotalMinor;
  final int deliveryFeeMinor;
  final int discountMinor;
  final int totalMinor;
  final String currency;
  final String? addressText;
  final String? note;
  final int prepTimeMin;
  final DateTime? readyAtEstimate;
  final String? cancellationReason;
  final DateTime createdAt;
  final DateTime? deliveredAt;
  final List<OrderItem> items;

  bool get isActive =>
      state != OrderState.delivered &&
      state != OrderState.cancelled &&
      state != OrderState.rejected;

  /// Anulimi lejohet vetëm para se kuzhina të nisë punën (§19).
  bool get canCancel => state == OrderState.pendingMerchant || state == OrderState.accepted;

  factory Order.fromJson(Map<String, dynamic> j) => Order(
    id: j['id'].toString(),
    code: (j['code'] ?? '').toString(),
    merchantId: (j['merchant_id'] ?? '').toString(),
    merchantName: (j['merchant_name'] ?? '').toString(),
    courierId: j['courier_id']?.toString(),
    state: orderStateFrom((j['state'] ?? 'pending_merchant').toString()),
    fulfillment: (j['fulfillment'] ?? 'courier').toString(),
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
    paymentStatus: (j['payment_status'] ?? 'none').toString(),
    itemsTotalMinor: (j['items_total_minor'] as num?)?.toInt() ?? 0,
    deliveryFeeMinor: (j['delivery_fee_minor'] as num?)?.toInt() ?? 0,
    discountMinor: (j['discount_minor'] as num?)?.toInt() ?? 0,
    totalMinor: (j['total_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    addressText: j['address_text']?.toString() ?? j['address_instructions']?.toString(),
    note: j['note']?.toString(),
    prepTimeMin: (j['prep_time_min'] as num?)?.toInt() ?? 0,
    readyAtEstimate: DateTime.tryParse(j['ready_at_estimate']?.toString() ?? ''),
    cancellationReason: j['cancellation_reason']?.toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
    deliveredAt: DateTime.tryParse(j['delivered_at']?.toString() ?? ''),
    items: ((j['items'] as List?) ?? const [])
        .map((e) => OrderItem.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
  );
}

class CourierOffer {
  CourierOffer({
    required this.id,
    required this.orderId,
    required this.code,
    required this.expiresAt,
    required this.distanceM,
    required this.etaS,
    required this.merchantName,
    required this.merchantAddress,
    required this.merchantLocation,
    this.dropoffAddress,
    required this.dropoff,
    required this.earningsMinor,
    required this.currency,
    required this.paymentMethod,
    required this.totalMinor,
  });

  final String id;
  final String orderId;
  final String code;
  final DateTime expiresAt;
  final int distanceM;
  final int etaS;
  final String merchantName;
  final String merchantAddress;
  final LatLng merchantLocation;
  final String? dropoffAddress;
  final LatLng dropoff;
  final int earningsMinor;
  final String currency;
  final String paymentMethod;
  final int totalMinor;

  int get secondsLeft {
    final left = expiresAt.difference(DateTime.now()).inSeconds;
    return left < 0 ? 0 : left;
  }

  bool get expired => secondsLeft == 0;

  factory CourierOffer.fromJson(Map<String, dynamic> j) => CourierOffer(
    id: j['id'].toString(),
    orderId: (j['order_id'] ?? '').toString(),
    code: (j['code'] ?? '').toString(),
    expiresAt:
        DateTime.tryParse(j['expires_at']?.toString() ?? '') ??
        DateTime.now().add(const Duration(seconds: 25)),
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    etaS: (j['eta_s'] as num?)?.toInt() ?? 0,
    merchantName: (j['merchant_name'] ?? '').toString(),
    merchantAddress: (j['merchant_address'] ?? '').toString(),
    merchantLocation: LatLng.fromJson(
      Map<String, dynamic>.from((j['merchant_location'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    dropoffAddress: j['dropoff_address']?.toString(),
    dropoff: LatLng.fromJson(
      Map<String, dynamic>.from((j['dropoff'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    earningsMinor: (j['earnings_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
    totalMinor: (j['total_minor'] as num?)?.toInt() ?? 0,
  );
}
