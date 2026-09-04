import 'ride.dart';

/// Gjendjet e një kërkese shërbimi, të njëjtat si te serveri (`services/state.go`).
enum ServiceState { open, booked, inProgress, completed, cancelled, noOffers }

ServiceState serviceStateFrom(String s) {
  switch (s) {
    case 'booked':
      return ServiceState.booked;
    case 'in_progress':
      return ServiceState.inProgress;
    case 'completed':
      return ServiceState.completed;
    case 'cancelled':
      return ServiceState.cancelled;
    case 'no_offers':
      return ServiceState.noOffers;
  }
  return ServiceState.open;
}

class ServiceCategory {
  const ServiceCategory({required this.id, required this.nameKey, required this.sort});

  final String id;
  final String nameKey;
  final int sort;

  factory ServiceCategory.fromJson(Map<String, dynamic> j) => ServiceCategory(
    id: (j['id'] ?? '').toString(),
    nameKey: (j['name_key'] ?? '').toString(),
    sort: (j['sort'] as num?)?.toInt() ?? 0,
  );
}

/// Mjeshtri siç e sheh klienti: emri, qyteti, vlerësimi. Telefoni privat nuk kalon kurrë këtu.
class ServiceProviderCard {
  const ServiceProviderCard({
    required this.userId,
    required this.name,
    this.businessName,
    required this.city,
    this.rating,
    required this.ratingCount,
    required this.jobsDone,
  });

  final String userId;
  final String name;
  final String? businessName;
  final String city;
  final double? rating;
  final int ratingCount;
  final int jobsDone;

  String get displayName {
    final b = businessName?.trim();
    if (b != null && b.isNotEmpty) return b;
    return name.isEmpty ? city : name;
  }

  factory ServiceProviderCard.fromJson(Map<String, dynamic> j) => ServiceProviderCard(
    userId: (j['user_id'] ?? '').toString(),
    name: (j['name'] ?? '').toString(),
    businessName: j['business_name']?.toString(),
    city: (j['city'] ?? '').toString(),
    rating: (j['rating'] as num?)?.toDouble(),
    ratingCount: (j['rating_count'] as num?)?.toInt() ?? 0,
    jobsDone: (j['jobs_done'] as num?)?.toInt() ?? 0,
  );
}

/// Oferta e një mjeshtri: çmimi e vendos ai, jo platforma.
class ServiceOffer {
  const ServiceOffer({
    required this.id,
    required this.requestId,
    required this.providerId,
    required this.priceMinor,
    required this.currency,
    this.note,
    this.canStartAt,
    required this.state,
    required this.createdAt,
    this.provider,
  });

  final String id;
  final String requestId;
  final String providerId;
  final int priceMinor;
  final String currency;
  final String? note;
  final DateTime? canStartAt;
  final String state;
  final DateTime createdAt;
  final ServiceProviderCard? provider;

  factory ServiceOffer.fromJson(Map<String, dynamic> j) => ServiceOffer(
    id: j['id'].toString(),
    requestId: (j['request_id'] ?? '').toString(),
    providerId: (j['provider_id'] ?? '').toString(),
    priceMinor: (j['price_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    note: j['note']?.toString(),
    canStartAt: DateTime.tryParse(j['can_start_at']?.toString() ?? ''),
    state: (j['state'] ?? 'offered').toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
    provider: j['provider'] is Map
        ? ServiceProviderCard.fromJson(Map<String, dynamic>.from(j['provider'] as Map))
        : null,
  );
}

class ServiceRequest {
  const ServiceRequest({
    required this.id,
    required this.code,
    required this.categoryId,
    this.providerId,
    required this.state,
    required this.title,
    required this.description,
    required this.addressLine1,
    required this.address,
    this.addressInstructions,
    this.preferredAt,
    required this.photoKeys,
    required this.paymentMethod,
    required this.paymentStatus,
    this.priceMinor,
    required this.currency,
    required this.createdAt,
    this.bookedAt,
    this.startedAt,
    this.completedAt,
    this.cancelledAt,
    this.provider,
    this.offers = const [],
  });

  final String id;
  final String code;
  final String categoryId;
  final String? providerId;
  final ServiceState state;
  final String title;
  final String description;
  final String addressLine1;
  final LatLng address;
  final String? addressInstructions;
  final DateTime? preferredAt;
  final List<String> photoKeys;
  final String paymentMethod;
  final String paymentStatus;
  final int? priceMinor;
  final String currency;
  final DateTime createdAt;
  final DateTime? bookedAt;
  final DateTime? startedAt;
  final DateTime? completedAt;
  final DateTime? cancelledAt;
  final ServiceProviderCard? provider;

  /// Ofertat vijnë vetëm te klienti dhe vetëm derisa ai të zgjedhë njërën.
  final List<ServiceOffer> offers;

  bool get isActive =>
      state == ServiceState.open ||
      state == ServiceState.booked ||
      state == ServiceState.inProgress;

  bool get isFinished => !isActive;

  bool get canCancel => state == ServiceState.open || state == ServiceState.booked;

  factory ServiceRequest.fromJson(Map<String, dynamic> j) => ServiceRequest(
    id: j['id'].toString(),
    code: (j['code'] ?? '').toString(),
    categoryId: (j['category_id'] ?? '').toString(),
    providerId: j['provider_id']?.toString(),
    state: serviceStateFrom((j['state'] ?? 'open').toString()),
    title: (j['title'] ?? '').toString(),
    description: (j['description'] ?? '').toString(),
    addressLine1: (j['address_line1'] ?? '').toString(),
    address: LatLng.fromJson(
      Map<String, dynamic>.from((j['address'] as Map?) ?? const {'lat': 0, 'lng': 0}),
    ),
    addressInstructions: j['address_instructions']?.toString(),
    preferredAt: DateTime.tryParse(j['preferred_at']?.toString() ?? ''),
    photoKeys: [for (final k in (j['photo_keys'] as List?) ?? const []) k.toString()],
    paymentMethod: (j['payment_method'] ?? 'cash').toString(),
    paymentStatus: (j['payment_status'] ?? 'none').toString(),
    priceMinor: (j['price_minor'] as num?)?.toInt(),
    currency: (j['currency'] ?? 'EUR').toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
    bookedAt: DateTime.tryParse(j['booked_at']?.toString() ?? ''),
    startedAt: DateTime.tryParse(j['started_at']?.toString() ?? ''),
    completedAt: DateTime.tryParse(j['completed_at']?.toString() ?? ''),
    cancelledAt: DateTime.tryParse(j['cancelled_at']?.toString() ?? ''),
    provider: j['provider'] is Map
        ? ServiceProviderCard.fromJson(Map<String, dynamic>.from(j['provider'] as Map))
        : null,
    offers: [
      for (final o in (j['offers'] as List?) ?? const [])
        ServiceOffer.fromJson(Map<String, dynamic>.from(o as Map)),
    ],
  );
}
