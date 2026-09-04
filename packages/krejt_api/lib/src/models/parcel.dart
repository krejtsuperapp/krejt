import 'ride.dart';

/// Gjendjet e pakos, të njëjtat si te serveri (`parcels/state.go`).
enum ParcelState { requested, courierAssigned, pickedUp, delivered, cancelled, noCourier }

ParcelState parcelStateFrom(String s) {
  switch (s) {
    case 'courier_assigned':
      return ParcelState.courierAssigned;
    case 'picked_up':
      return ParcelState.pickedUp;
    case 'delivered':
      return ParcelState.delivered;
    case 'cancelled':
      return ParcelState.cancelled;
    case 'no_courier':
      return ParcelState.noCourier;
  }
  return ParcelState.requested;
}

/// Çmimi i dërgesës, server-side, vlen dy minuta.
class ParcelQuote {
  const ParcelQuote({
    required this.id,
    required this.size,
    required this.distanceM,
    required this.durationS,
    required this.priceMinor,
    this.discountMinor = 0,
    required this.currency,
    required this.expiresAt,
  });

  final String id;
  final String size;
  final int distanceM;
  final int durationS;
  final int priceMinor;

  /// Zbritja e kuponit; klienti paguan priceMinor − discountMinor.
  final int discountMinor;
  final String currency;
  final DateTime expiresAt;

  bool get expired => DateTime.now().isAfter(expiresAt);

  factory ParcelQuote.fromJson(Map<String, dynamic> j) => ParcelQuote(
    id: j['id'].toString(),
    size: (j['size'] ?? 's').toString(),
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    durationS: (j['duration_s'] as num?)?.toInt() ?? 0,
    priceMinor: (j['price_minor'] as num?)?.toInt() ?? 0,
    discountMinor: (j['discount_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    expiresAt: DateTime.tryParse(j['expires_at']?.toString() ?? '') ?? DateTime.now(),
  );
}

class ParcelCourier {
  const ParcelCourier({
    required this.id,
    required this.name,
    required this.vehicleMake,
    required this.vehicleModel,
    required this.vehiclePlate,
    required this.vehicleColor,
  });

  final String id;
  final String name;
  final String vehicleMake;
  final String vehicleModel;
  final String vehiclePlate;
  final String vehicleColor;

  String get vehicle => '$vehicleColor $vehicleMake $vehicleModel'.trim();

  factory ParcelCourier.fromJson(Map<String, dynamic> j) => ParcelCourier(
    id: (j['id'] ?? '').toString(),
    name: (j['name'] ?? '').toString(),
    vehicleMake: (j['vehicle_make'] ?? '').toString(),
    vehicleModel: (j['vehicle_model'] ?? '').toString(),
    vehiclePlate: (j['vehicle_plate'] ?? '').toString(),
    vehicleColor: (j['vehicle_color'] ?? '').toString(),
  );
}

/// Një dërgesë pakoje. Kodet e marrjes/dorëzimit vijnë vetëm te klienti; korrieri i merr gojarisht.
class Parcel {
  const Parcel({
    required this.id,
    required this.code,
    this.pickupCode,
    this.deliveryCode,
    this.courierId,
    required this.state,
    required this.size,
    required this.paymentMethod,
    required this.paymentStatus,
    required this.pickup,
    this.pickupAddress,
    this.pickupContactName,
    this.pickupContactPhone,
    required this.dropoff,
    this.dropoffAddress,
    required this.recipientName,
    required this.recipientPhone,
    this.note,
    required this.distanceM,
    required this.durationS,
    required this.priceMinor,
    required this.currency,
    required this.createdAt,
    this.assignedAt,
    this.pickedUpAt,
    this.deliveredAt,
    this.cancelledAt,
    this.courier,
  });

  final String id;
  final String code;
  final String? pickupCode;
  final String? deliveryCode;
  final String? courierId;
  final ParcelState state;
  final String size;
  final String paymentMethod;
  final String paymentStatus;
  final LatLng pickup;
  final String? pickupAddress;
  final String? pickupContactName;
  final String? pickupContactPhone;
  final LatLng dropoff;
  final String? dropoffAddress;
  final String recipientName;
  final String recipientPhone;
  final String? note;
  final int distanceM;
  final int durationS;
  final int priceMinor;
  final String currency;
  final DateTime createdAt;
  final DateTime? assignedAt;
  final DateTime? pickedUpAt;
  final DateTime? deliveredAt;
  final DateTime? cancelledAt;
  final ParcelCourier? courier;

  bool get isActive =>
      state == ParcelState.requested ||
      state == ParcelState.courierAssigned ||
      state == ParcelState.pickedUp;

  bool get isFinished => !isActive;

  /// Anulimi lejohet derisa korrieri ta marrë pakon.
  bool get canCancel => state == ParcelState.requested || state == ParcelState.courierAssigned;

  factory Parcel.fromJson(Map<String, dynamic> j) => Parcel(
    id: j['id'].toString(),
    code: (j['code'] ?? '').toString(),
    pickupCode: j['pickup_code']?.toString(),
    deliveryCode: j['delivery_code']?.toString(),
    courierId: j['courier_id']?.toString(),
    state: parcelStateFrom((j['state'] ?? 'requested').toString()),
    size: (j['size'] ?? 's').toString(),
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
    paymentStatus: (j['payment_status'] ?? 'pending').toString(),
    pickup: LatLng.fromJson(
      Map<String, dynamic>.from((j['pickup'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    pickupAddress: j['pickup_address']?.toString(),
    pickupContactName: j['pickup_contact_name']?.toString(),
    pickupContactPhone: j['pickup_contact_phone']?.toString(),
    dropoff: LatLng.fromJson(
      Map<String, dynamic>.from((j['dropoff'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    dropoffAddress: j['dropoff_address']?.toString(),
    recipientName: (j['recipient_name'] ?? '').toString(),
    recipientPhone: (j['recipient_phone'] ?? '').toString(),
    note: j['note']?.toString(),
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    durationS: (j['duration_s'] as num?)?.toInt() ?? 0,
    priceMinor: (j['price_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
    assignedAt: DateTime.tryParse(j['assigned_at']?.toString() ?? ''),
    pickedUpAt: DateTime.tryParse(j['picked_up_at']?.toString() ?? ''),
    deliveredAt: DateTime.tryParse(j['delivered_at']?.toString() ?? ''),
    cancelledAt: DateTime.tryParse(j['cancelled_at']?.toString() ?? ''),
    courier: j['courier'] is Map
        ? ParcelCourier.fromJson(Map<String, dynamic>.from(j['courier'] as Map))
        : null,
  );
}

/// Oferta e një pakoje siç e sheh korrieri (pa kode).
class ParcelOffer {
  const ParcelOffer({
    required this.id,
    required this.parcelId,
    required this.code,
    required this.expiresAt,
    required this.distanceM,
    required this.etaS,
    required this.size,
    this.pickupAddress,
    required this.pickup,
    this.dropoffAddress,
    required this.dropoff,
    required this.routeM,
    required this.earningsMinor,
    required this.currency,
    required this.paymentMethod,
    required this.totalMinor,
  });

  final String id;
  final String parcelId;
  final String code;
  final DateTime expiresAt;
  final int distanceM;
  final int etaS;
  final String size;
  final String? pickupAddress;
  final LatLng pickup;
  final String? dropoffAddress;
  final LatLng dropoff;
  final int routeM;
  final int earningsMinor;
  final String currency;
  final String paymentMethod;
  final int totalMinor;

  int get secondsLeft {
    final s = expiresAt.difference(DateTime.now()).inSeconds;
    return s < 0 ? 0 : s;
  }

  factory ParcelOffer.fromJson(Map<String, dynamic> j) => ParcelOffer(
    id: j['id'].toString(),
    parcelId: (j['parcel_id'] ?? '').toString(),
    code: (j['code'] ?? '').toString(),
    expiresAt: DateTime.tryParse(j['expires_at']?.toString() ?? '') ?? DateTime.now(),
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    etaS: (j['eta_s'] as num?)?.toInt() ?? 0,
    size: (j['size'] ?? 's').toString(),
    pickupAddress: j['pickup_address']?.toString(),
    pickup: LatLng.fromJson(
      Map<String, dynamic>.from((j['pickup'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    dropoffAddress: j['dropoff_address']?.toString(),
    dropoff: LatLng.fromJson(
      Map<String, dynamic>.from((j['dropoff'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    routeM: (j['route_m'] as num?)?.toInt() ?? 0,
    earningsMinor: (j['earnings_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
    totalMinor: (j['total_minor'] as num?)?.toInt() ?? 0,
  );
}
