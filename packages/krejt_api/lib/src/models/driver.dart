import 'ride.dart';

/// Modelet e shoferit dhe korrierit (§27, §28, §30, §31).

enum DriverStatus { pending, approved, suspended }

class DriverProfile {
  DriverProfile({
    required this.userId,
    required this.status,
    required this.vehicleMake,
    required this.vehicleModel,
    required this.vehiclePlate,
    required this.vehicleColor,
    required this.categories,
    this.rating,
    required this.ratingCount,
    this.suspendedReason,
  });

  final String userId;
  final DriverStatus status;
  final String vehicleMake;
  final String vehicleModel;
  final String vehiclePlate;
  final String vehicleColor;
  final List<RideCategory> categories;
  final double? rating;
  final int ratingCount;
  final String? suspendedReason;

  bool get canGoOnline => status == DriverStatus.approved;
  String get vehicle => '$vehicleColor $vehicleMake $vehicleModel'.trim();

  factory DriverProfile.fromJson(Map<String, dynamic> j) => DriverProfile(
    userId: (j['user_id'] ?? '').toString(),
    status: DriverStatus.values.firstWhere(
      (s) => s.name == (j['status'] ?? 'pending').toString(),
      orElse: () => DriverStatus.pending,
    ),
    vehicleMake: (j['vehicle_make'] ?? '').toString(),
    vehicleModel: (j['vehicle_model'] ?? '').toString(),
    vehiclePlate: (j['vehicle_plate'] ?? '').toString(),
    vehicleColor: (j['vehicle_color'] ?? '').toString(),
    categories: ((j['categories'] as List?) ?? const [])
        .map((e) => rideCategoryFrom(e.toString()))
        .toList(),
    rating: (j['rating'] as num?)?.toDouble(),
    ratingCount: (j['rating_count'] as num?)?.toInt() ?? 0,
    suspendedReason: j['suspended_reason']?.toString(),
  );
}

class RideOffer {
  RideOffer({
    required this.id,
    required this.rideId,
    required this.round,
    required this.expiresAt,
    required this.distanceM,
    required this.etaS,
    required this.category,
    required this.pickup,
    this.pickupAddress,
    required this.dropoff,
    this.dropoffAddress,
    required this.rideDistanceM,
    required this.rideDurationS,
    required this.priceMinor,
    required this.earningsMinor,
    required this.currency,
    required this.paymentMethod,
  });

  final String id;
  final String rideId;
  final int round;
  final DateTime expiresAt;

  /// Distanca deri te marrja (jo gjatësia e udhëtimit).
  final int distanceM;
  final int etaS;
  final RideCategory category;
  final LatLng pickup;
  final String? pickupAddress;
  final LatLng dropoff;
  final String? dropoffAddress;
  final int rideDistanceM;
  final int rideDurationS;
  final int priceMinor;
  final int earningsMinor;
  final String currency;
  final String paymentMethod;

  int get secondsLeft {
    final d = expiresAt.difference(DateTime.now()).inSeconds;
    return d < 0 ? 0 : d;
  }

  bool get expired => secondsLeft == 0;

  factory RideOffer.fromJson(Map<String, dynamic> j) => RideOffer(
    id: j['id'].toString(),
    rideId: j['ride_id'].toString(),
    round: (j['round'] as num?)?.toInt() ?? 1,
    expiresAt:
        DateTime.tryParse(j['expires_at']?.toString() ?? '') ??
        DateTime.now().add(const Duration(seconds: 15)),
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    etaS: (j['eta_s'] as num?)?.toInt() ?? 0,
    category: rideCategoryFrom((j['category'] ?? 'economy').toString()),
    pickup: LatLng.fromJson(
      Map<String, dynamic>.from((j['pickup'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    pickupAddress: j['pickup_address']?.toString(),
    dropoff: LatLng.fromJson(
      Map<String, dynamic>.from((j['dropoff'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    dropoffAddress: j['dropoff_address']?.toString(),
    rideDistanceM: (j['ride_distance_m'] as num?)?.toInt() ?? 0,
    rideDurationS: (j['ride_duration_s'] as num?)?.toInt() ?? 0,
    priceMinor: (j['price_minor'] as num?)?.toInt() ?? 0,
    earningsMinor: (j['earnings_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
  );
}

class Earnings {
  Earnings({
    required this.balanceMinor,
    required this.todayMinor,
    required this.weekMinor,
    required this.monthMinor,
    required this.ridesToday,
    required this.ridesWeek,
    required this.cashCollectedWeekMinor,
    required this.nextPayoutMinMinor,
    required this.currency,
  });

  final int balanceMinor;
  final int todayMinor;
  final int weekMinor;
  final int monthMinor;
  final int ridesToday;
  final int ridesWeek;
  final int cashCollectedWeekMinor;
  final int nextPayoutMinMinor;
  final String currency;

  bool get payoutReady => balanceMinor >= nextPayoutMinMinor;

  factory Earnings.fromJson(Map<String, dynamic> j) => Earnings(
    balanceMinor: (j['balance_minor'] as num?)?.toInt() ?? 0,
    todayMinor: (j['today_minor'] as num?)?.toInt() ?? 0,
    weekMinor: (j['week_minor'] as num?)?.toInt() ?? 0,
    monthMinor: (j['month_minor'] as num?)?.toInt() ?? 0,
    ridesToday: (j['rides_today'] as num?)?.toInt() ?? 0,
    ridesWeek: (j['rides_week'] as num?)?.toInt() ?? 0,
    cashCollectedWeekMinor: (j['cash_collected_week_minor'] as num?)?.toInt() ?? 0,
    nextPayoutMinMinor: (j['next_payout_min_minor'] as num?)?.toInt() ?? 2000,
    currency: (j['currency'] ?? 'EUR').toString(),
  );
}

class BankAccount {
  BankAccount({required this.holderName, required this.ibanMasked, this.bankName, this.verifiedAt});

  final String holderName;
  final String ibanMasked;
  final String? bankName;
  final DateTime? verifiedAt;

  bool get verified => verifiedAt != null;

  factory BankAccount.fromJson(Map<String, dynamic> j) => BankAccount(
    holderName: (j['holder_name'] ?? '').toString(),
    ibanMasked: (j['iban_masked'] ?? '').toString(),
    bankName: j['bank_name']?.toString(),
    verifiedAt: DateTime.tryParse(j['verified_at']?.toString() ?? ''),
  );
}

enum DocumentStatus { pending, approved, rejected, expired, replaced }

class DriverDocument {
  DriverDocument({
    required this.id,
    required this.type,
    required this.status,
    this.expiresOn,
    this.rejectionReason,
    required this.createdAt,
  });

  final String id;
  final String type;
  final DocumentStatus status;
  final DateTime? expiresOn;
  final String? rejectionReason;
  final DateTime createdAt;

  factory DriverDocument.fromJson(Map<String, dynamic> j) => DriverDocument(
    id: j['id'].toString(),
    type: (j['type'] ?? '').toString(),
    status: DocumentStatus.values.firstWhere(
      (s) => s.name == (j['status'] ?? 'pending').toString(),
      orElse: () => DocumentStatus.pending,
    ),
    expiresOn: DateTime.tryParse(j['expires_on']?.toString() ?? ''),
    rejectionReason: j['rejection_reason']?.toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
  );
}

class DocumentsOverview {
  DocumentsOverview({
    required this.documents,
    required this.missing,
    required this.expiring,
    required this.eligible,
  });

  final List<DriverDocument> documents;
  final List<String> missing;
  final List<String> expiring;
  final bool eligible;

  factory DocumentsOverview.fromJson(Map<String, dynamic> j) => DocumentsOverview(
    documents: ((j['documents'] as List?) ?? const [])
        .map((e) => DriverDocument.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
    missing: ((j['missing'] as List?) ?? const []).map((e) => e.toString()).toList(),
    expiring: ((j['expiring'] as List?) ?? const []).map((e) => e.toString()).toList(),
    eligible: j['eligible'] == true,
  );
}
