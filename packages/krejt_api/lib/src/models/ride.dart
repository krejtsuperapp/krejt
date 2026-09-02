/// Modelet e udhëtimit (§20–§26). Çmimi, gjendja dhe kodi i marrjes vijnë vetëm nga serveri.
library;

class LatLng {
  const LatLng(this.lat, this.lng);

  final double lat;
  final double lng;

  factory LatLng.fromJson(Map<String, dynamic> j) =>
      LatLng((j['lat'] as num).toDouble(), (j['lng'] as num).toDouble());

  Map<String, dynamic> toJson() => {'lat': lat, 'lng': lng};

  @override
  bool operator ==(Object other) => other is LatLng && other.lat == lat && other.lng == lng;

  @override
  int get hashCode => Object.hash(lat, lng);
}

enum RideCategory { economy, comfort, xl, taxi }

RideCategory rideCategoryFrom(String s) =>
    RideCategory.values.firstWhere((c) => c.name == s, orElse: () => RideCategory.economy);

enum RideState { matching, assigned, arrived, inProgress, completed, cancelled, noDriver }

RideState rideStateFrom(String s) {
  switch (s) {
    case 'matching':
      return RideState.matching;
    case 'assigned':
      return RideState.assigned;
    case 'arrived':
      return RideState.arrived;
    case 'in_progress':
      return RideState.inProgress;
    case 'completed':
      return RideState.completed;
    case 'cancelled':
      return RideState.cancelled;
    default:
      return RideState.noDriver;
  }
}

class RideQuote {
  RideQuote({
    required this.id,
    required this.category,
    required this.seats,
    required this.priceMinor,
    required this.currency,
    required this.surgeBp,
    this.pickupEtaS,
    required this.expiresAt,
  });

  final String id;
  final RideCategory category;
  final int seats;
  final int priceMinor;
  final String currency;
  final int surgeBp;
  final int? pickupEtaS;
  final DateTime expiresAt;

  bool get surging => surgeBp > 10000;
  bool get expired => DateTime.now().isAfter(expiresAt);

  factory RideQuote.fromJson(Map<String, dynamic> j) => RideQuote(
    id: j['id'].toString(),
    category: rideCategoryFrom((j['category'] ?? 'economy').toString()),
    seats: (j['seats'] as num?)?.toInt() ?? 4,
    priceMinor: (j['price_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    surgeBp: (j['surge_bp'] as num?)?.toInt() ?? 10000,
    pickupEtaS: (j['pickup_eta_s'] as num?)?.toInt(),
    expiresAt:
        DateTime.tryParse(j['expires_at']?.toString() ?? '') ??
        DateTime.now().add(const Duration(minutes: 2)),
  );
}

class QuoteResult {
  QuoteResult({
    this.areaName,
    required this.distanceM,
    required this.durationS,
    required this.quotes,
  });

  final String? areaName;
  final int distanceM;
  final int durationS;
  final List<RideQuote> quotes;

  factory QuoteResult.fromJson(Map<String, dynamic> j) => QuoteResult(
    areaName: j['area'] is Map ? (j['area'] as Map)['name']?.toString() : null,
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    durationS: (j['duration_s'] as num?)?.toInt() ?? 0,
    quotes: ((j['quotes'] as List?) ?? const [])
        .map((e) => RideQuote.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
  );
}

class DriverCard {
  DriverCard({
    required this.id,
    this.name,
    required this.vehicleMake,
    required this.vehicleModel,
    required this.vehiclePlate,
    required this.vehicleColor,
    this.rating,
    this.location,
    this.locationAt,
  });

  final String id;
  final String? name;
  final String vehicleMake;
  final String vehicleModel;
  final String vehiclePlate;
  final String vehicleColor;
  final double? rating;
  final LatLng? location;
  final DateTime? locationAt;

  String get vehicle => '$vehicleColor $vehicleMake $vehicleModel'.trim();

  factory DriverCard.fromJson(Map<String, dynamic> j) => DriverCard(
    id: j['id'].toString(),
    name: j['name']?.toString(),
    vehicleMake: (j['vehicle_make'] ?? '').toString(),
    vehicleModel: (j['vehicle_model'] ?? '').toString(),
    vehiclePlate: (j['vehicle_plate'] ?? '').toString(),
    vehicleColor: (j['vehicle_color'] ?? '').toString(),
    rating: (j['rating'] as num?)?.toDouble(),
    location: j['location'] is Map
        ? LatLng.fromJson(Map<String, dynamic>.from(j['location'] as Map))
        : null,
    locationAt: DateTime.tryParse(j['location_at']?.toString() ?? ''),
  );
}

class Ride {
  Ride({
    required this.id,
    this.driverId,
    required this.category,
    required this.state,
    required this.paymentMethod,
    required this.paymentStatus,
    required this.pickup,
    this.pickupAddress,
    this.pickupCode,
    required this.dropoff,
    this.dropoffAddress,
    required this.distanceM,
    required this.durationS,
    required this.priceQuotedMinor,
    this.priceFinalMinor,
    required this.cancellationFeeMinor,
    required this.currency,
    this.note,
    this.cancelledBy,
    this.cancellationReason,
    required this.requestedAt,
    this.assignedAt,
    this.arrivedAt,
    this.startedAt,
    this.completedAt,
    this.cancelledAt,
    this.driver,
  });

  final String id;
  final String? driverId;
  final RideCategory category;
  final RideState state;
  final String paymentMethod;
  final String paymentStatus;
  final LatLng pickup;
  final String? pickupAddress;
  final String? pickupCode;
  final LatLng dropoff;
  final String? dropoffAddress;
  final int distanceM;
  final int durationS;
  final int priceQuotedMinor;
  final int? priceFinalMinor;
  final int cancellationFeeMinor;
  final String currency;
  final String? note;
  final String? cancelledBy;
  final String? cancellationReason;
  final DateTime requestedAt;
  final DateTime? assignedAt;
  final DateTime? arrivedAt;
  final DateTime? startedAt;
  final DateTime? completedAt;
  final DateTime? cancelledAt;
  final DriverCard? driver;

  bool get isActive =>
      state == RideState.matching ||
      state == RideState.assigned ||
      state == RideState.arrived ||
      state == RideState.inProgress;

  bool get isFinished =>
      state == RideState.completed || state == RideState.cancelled || state == RideState.noDriver;

  /// Çmimi për shfaqje: final kur ekziston, ndryshe ai i ofertës.
  int get priceMinor => priceFinalMinor ?? priceQuotedMinor;

  /// Biseda është e hapur nga caktimi deri 24 h pas përfundimit (§26).
  bool get chatOpen {
    if (driverId == null) return false;
    final done = completedAt ?? cancelledAt;
    if (done == null) return true;
    return DateTime.now().difference(done) < const Duration(hours: 24);
  }

  factory Ride.fromJson(Map<String, dynamic> j) => Ride(
    id: j['id'].toString(),
    driverId: j['driver_id']?.toString(),
    category: rideCategoryFrom((j['category'] ?? 'economy').toString()),
    state: rideStateFrom((j['state'] ?? 'matching').toString()),
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
    paymentStatus: (j['payment_status'] ?? 'none').toString(),
    pickup: LatLng.fromJson(
      Map<String, dynamic>.from((j['pickup'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    pickupAddress: j['pickup_address']?.toString(),
    pickupCode: j['pickup_code']?.toString(),
    dropoff: LatLng.fromJson(
      Map<String, dynamic>.from((j['dropoff'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    dropoffAddress: j['dropoff_address']?.toString(),
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    durationS: (j['duration_s'] as num?)?.toInt() ?? 0,
    priceQuotedMinor: (j['price_quoted_minor'] as num?)?.toInt() ?? 0,
    priceFinalMinor: (j['price_final_minor'] as num?)?.toInt(),
    cancellationFeeMinor: (j['cancellation_fee_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    note: j['note']?.toString(),
    cancelledBy: j['cancelled_by']?.toString(),
    cancellationReason: j['cancellation_reason']?.toString(),
    requestedAt: DateTime.tryParse(j['requested_at']?.toString() ?? '') ?? DateTime.now(),
    assignedAt: DateTime.tryParse(j['assigned_at']?.toString() ?? ''),
    arrivedAt: DateTime.tryParse(j['arrived_at']?.toString() ?? ''),
    startedAt: DateTime.tryParse(j['started_at']?.toString() ?? ''),
    completedAt: DateTime.tryParse(j['completed_at']?.toString() ?? ''),
    cancelledAt: DateTime.tryParse(j['cancelled_at']?.toString() ?? ''),
    driver: j['driver'] is Map
        ? DriverCard.fromJson(Map<String, dynamic>.from(j['driver'] as Map))
        : null,
  );
}

class ChatMessage {
  ChatMessage({
    required this.id,
    required this.rideId,
    required this.senderRole,
    required this.body,
    required this.createdAt,
    this.readAt,
    required this.mine,
  });

  final String id;
  final String rideId;
  final String senderRole;
  final String body;
  final DateTime createdAt;
  final DateTime? readAt;
  final bool mine;

  factory ChatMessage.fromJson(Map<String, dynamic> j) => ChatMessage(
    id: j['id'].toString(),
    rideId: j['ride_id'].toString(),
    senderRole: (j['sender_role'] ?? 'customer').toString(),
    body: (j['body'] ?? '').toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
    readAt: DateTime.tryParse(j['read_at']?.toString() ?? ''),
    mine: j['mine'] == true,
  );
}

class PickupToken {
  PickupToken({required this.token, required this.expiresAt});

  final String token;
  final DateTime expiresAt;

  factory PickupToken.fromJson(Map<String, dynamic> j) => PickupToken(
    token: (j['token'] ?? '').toString(),
    expiresAt:
        DateTime.tryParse(j['expires_at']?.toString() ?? '') ??
        DateTime.now().add(const Duration(minutes: 5)),
  );
}
