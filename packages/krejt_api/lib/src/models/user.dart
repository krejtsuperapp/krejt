/// Modelet e identitetit dhe llogarisë (§16, §53). Fushat pasqyrojnë skemat `Me`, `Address`, `Session`.
library;

class TokenPair {
  TokenPair({
    required this.accessToken,
    required this.refreshToken,
    this.expiresAt,
    this.userId,
    this.isNewUser = false,
  });

  final String accessToken;
  final String refreshToken;
  final DateTime? expiresAt;
  final String? userId;
  final bool isNewUser;

  factory TokenPair.fromJson(Map<String, dynamic> j) => TokenPair(
    accessToken: j['access_token'] as String,
    refreshToken: j['refresh_token'] as String,
    expiresAt: j['expires_at'] == null ? null : DateTime.tryParse(j['expires_at'].toString()),
    userId: j['user_id']?.toString(),
    isNewUser: j['is_new_user'] == true,
  );
}

class Wallet {
  Wallet({required this.balanceMinor, required this.currency, this.closedLoop = true});

  final int balanceMinor;
  final String currency;
  final bool closedLoop;

  factory Wallet.fromJson(Map<String, dynamic> j) => Wallet(
    balanceMinor: (j['balance_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    closedLoop: j['closed_loop'] != false,
  );
}

class Me {
  Me({
    required this.id,
    this.phone,
    this.email,
    this.fullName,
    required this.locale,
    required this.capabilities,
    required this.wallet,
    this.photoUrl,
  });

  final String id;
  final String? phone;
  final String? email;
  final String? fullName;
  final String locale;
  final List<String> capabilities;
  final Wallet wallet;

  /// Publike (CloudFront); null pa foto profili.
  final String? photoUrl;

  bool get isDriver => capabilities.contains('RIDE_DRIVER') || capabilities.contains('TAXI_DRIVER');
  bool get isMerchant => capabilities.contains('MERCHANT');

  /// Emri i shfaqur: emri i plotë nëse ekziston, ndryshe telefoni i maskuar.
  String get displayName {
    final n = fullName?.trim();
    if (n != null && n.isNotEmpty) return n;
    final p = phone;
    if (p == null || p.length < 4) return '—';
    return '•••${p.substring(p.length - 3)}';
  }

  String get initials {
    final n = fullName?.trim();
    if (n == null || n.isEmpty) return 'K';
    final parts = n.split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
    final first = parts.first.substring(0, 1);
    final second = parts.length > 1 ? parts[1].substring(0, 1) : '';
    return (first + second).toUpperCase();
  }

  factory Me.fromJson(Map<String, dynamic> j) => Me(
    id: j['id'].toString(),
    phone: j['phone']?.toString(),
    email: j['email']?.toString(),
    fullName: j['full_name']?.toString(),
    locale: (j['locale'] ?? 'sq').toString(),
    capabilities: ((j['capabilities'] as List?) ?? const []).map((e) => e.toString()).toList(),
    wallet: Wallet.fromJson(Map<String, dynamic>.from((j['wallet'] as Map?) ?? const {})),
    photoUrl: j['photo_url']?.toString(),
  );
}

class Address {
  Address({
    required this.id,
    required this.label,
    this.name,
    required this.line1,
    this.line2,
    required this.city,
    required this.lat,
    required this.lng,
    this.instructions,
    this.isDefault = false,
  });

  final String id;
  final String label; // home | work | other
  final String? name;
  final String line1;
  final String? line2;
  final String city;
  final double lat;
  final double lng;
  final String? instructions;
  final bool isDefault;

  factory Address.fromJson(Map<String, dynamic> j) => Address(
    id: j['id'].toString(),
    label: (j['label'] ?? 'other').toString(),
    name: j['name']?.toString(),
    line1: (j['line1'] ?? '').toString(),
    line2: j['line2']?.toString(),
    city: (j['city'] ?? '').toString(),
    lat: (j['lat'] as num).toDouble(),
    lng: (j['lng'] as num).toDouble(),
    instructions: j['instructions']?.toString(),
    isDefault: j['is_default'] == true,
  );

  Map<String, dynamic> toJson() => {
    'label': label,
    if (name != null) 'name': name,
    'line1': line1,
    if (line2 != null) 'line2': line2,
    'city': city,
    'lat': lat,
    'lng': lng,
    if (instructions != null) 'instructions': instructions,
    'is_default': isDefault,
  };
}

class DeviceSession {
  DeviceSession({
    required this.id,
    this.deviceName,
    this.platform,
    this.ip,
    required this.lastSeenAt,
    required this.current,
  });

  final String id;
  final String? deviceName;
  final String? platform;
  final String? ip;
  final DateTime lastSeenAt;
  final bool current;

  factory DeviceSession.fromJson(Map<String, dynamic> j) => DeviceSession(
    id: j['id'].toString(),
    deviceName: j['device_name']?.toString(),
    platform: j['platform']?.toString(),
    ip: j['ip']?.toString(),
    lastSeenAt: DateTime.tryParse(j['last_seen_at']?.toString() ?? '') ?? DateTime.now(),
    current: j['current'] == true,
  );
}

class NotificationPreference {
  NotificationPreference({
    required this.category,
    required this.push,
    required this.email,
    required this.sms,
  });

  final String category;
  final bool push;
  final bool email;
  final bool sms;

  bool get locked => category == 'security'; // §51: sinjalet e sigurisë nuk çaktivizohen

  factory NotificationPreference.fromJson(Map<String, dynamic> j) => NotificationPreference(
    category: j['category'].toString(),
    push: j['push'] == true,
    email: j['email'] == true,
    sms: j['sms'] == true,
  );

  Map<String, dynamic> toJson() => {'category': category, 'push': push, 'email': email, 'sms': sms};

  NotificationPreference copyWith({bool? push, bool? email, bool? sms}) => NotificationPreference(
    category: category,
    push: push ?? this.push,
    email: email ?? this.email,
    sms: sms ?? this.sms,
  );
}
