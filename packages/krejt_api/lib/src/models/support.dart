/// Tiketat e mbështetjes. Një bisedë e vetme për çdo çështje: përdoruesi shkruan, agjenti përgjigjet,
/// dhe historia mbetet aty derisa çështja të mbyllet.
class TicketMessage {
  const TicketMessage({
    required this.id,
    required this.authorRole,
    required this.body,
    required this.createdAt,
  });

  factory TicketMessage.fromJson(Map<String, dynamic> j) => TicketMessage(
    id: j['id'] as String,
    authorRole: j['author_role'] as String? ?? 'user',
    body: j['body'] as String? ?? '',
    createdAt: DateTime.parse(j['created_at'] as String).toLocal(),
  );

  final String id;

  /// `user` ose `agent`; përcakton anën e flluskës në bisedë.
  final String authorRole;
  final String body;
  final DateTime createdAt;

  bool get mine => authorRole == 'user';
}

class SupportTicket {
  const SupportTicket({
    required this.id,
    required this.category,
    required this.subject,
    required this.status,
    required this.priority,
    required this.lastMessageAt,
    required this.createdAt,
    this.rideId,
    this.messages = const [],
  });

  factory SupportTicket.fromJson(Map<String, dynamic> j) => SupportTicket(
    id: j['id'] as String,
    category: j['category'] as String? ?? 'other',
    subject: j['subject'] as String? ?? '',
    status: j['status'] as String? ?? 'open',
    priority: j['priority'] as String? ?? 'normal',
    rideId: j['ride_id'] as String?,
    lastMessageAt: DateTime.parse(j['last_message_at'] as String).toLocal(),
    createdAt: DateTime.parse(j['created_at'] as String).toLocal(),
    messages: ((j['messages'] as List?) ?? const [])
        .map((m) => TicketMessage.fromJson(m as Map<String, dynamic>))
        .toList(),
  );

  final String id;

  /// ride | order | payment | refund | account | safety | other
  final String category;
  final String subject;

  /// open | pending_user | resolved | closed
  final String status;
  final String priority;
  final String? rideId;
  final DateTime lastMessageAt;
  final DateTime createdAt;
  final List<TicketMessage> messages;

  /// E mbyllur do të thotë pa përgjigje të re: aplikacioni e fsheh fushën e shkrimit që përdoruesi
  /// të mos shkruajë diçka që nuk do ta lexojë askush.
  bool get closed => status == 'closed';
}

/// Kategoritë që serveri pranon, në radhën si i sheh përdoruesi. 'safety' rri brenda me qëllim:
/// pa të, një problem sigurie nuk do të kishte fare rrugë; serveri ia vë vetë përparësinë urgjente.
const supportCategories = <String>[
  'ride',
  'order',
  'payment',
  'refund',
  'safety',
  'account',
  'other',
];
