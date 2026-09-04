import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'food/order_tracking.dart';
import 'parcels/parcel_tracking.dart';
import 'ride/tracking.dart';
import 'services/request_tracking.dart';

/// Ikona e njoftimeve me numëratorin e palexuarave. Numëratori vjen nga serveri me çdo rifreskim
/// të ballinës; pa njoftime të palexuara nuk shfaqet asnjë pikë, që zilja të mos bërtasë kot.
class NotificationsButton extends StatelessWidget {
  const NotificationsButton({super.key});

  @override
  Widget build(BuildContext context) {
    final unread = context.watch<AppState>().unreadNotifications;
    return Semantics(
      button: true,
      label: context.t('notifications.title'),
      child: InkWell(
        borderRadius: BorderRadius.circular(K.rFull),
        onTap: () =>
            Navigator.of(context)
                .push(MaterialPageRoute<void>(builder: (_) => const NotificationsScreen())),
        child: Container(
          width: 42,
          height: 42,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: K.surface,
            borderRadius: BorderRadius.circular(K.rFull),
            border: Border.all(color: K.line2),
          ),
          child: Stack(
            clipBehavior: Clip.none,
            children: [
              const Icon(Icons.notifications_none, size: 21, color: K.textDim),
              if (unread > 0)
                Positioned(
                  right: -3,
                  top: -3,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 4),
                    constraints: const BoxConstraints(minWidth: 15, minHeight: 15),
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: K.brand500,
                      borderRadius: BorderRadius.circular(K.rFull),
                      boxShadow: [
                        BoxShadow(color: K.brand500.withValues(alpha: 0.6), blurRadius: 8),
                      ],
                    ),
                    child: Text(
                      unread > 9 ? '9+' : '$unread',
                      style: const TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.w800,
                        color: K.onBrand,
                        height: 1.1,
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Kutia e njoftimeve. Teksti përkthehet në pajisje nga çelësi dhe parametrat e serverit, ndaj
/// ndryshimi i gjuhës e ndryshon edhe historikun (§2). Prekja hap atë që njoftimi përshkruan.
class NotificationsScreen extends StatefulWidget {
  const NotificationsScreen({super.key});

  @override
  State<NotificationsScreen> createState() => _NotificationsScreenState();
}

class _NotificationsScreenState extends State<NotificationsScreen> {
  List<AppNotification> _items = const [];
  ApiError? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final items = await context.read<AppState>().api.notifications();
      if (!mounted) return;
      setState(() {
        _items = items;
        _error = null;
        _loading = false;
      });
      if (mounted) context.read<AppState>().setUnread(items.where((n) => n.unread).length);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  Future<void> _readAll() async {
    final state = context.read<AppState>();
    try {
      await state.api.markAllNotificationsRead();
    } on ApiError {
      // Shenja e leximit nuk është kritike; lista rifreskohet gjithsesi.
    }
    await _load();
  }

  /// Deep link-u i serverit hapet brenda aplikacionit; adresat që s'i njohim nuk bëjnë asgjë,
  /// në vend që të hapin një ekran të gabuar.
  Future<void> _open(AppNotification n) async {
    final state = context.read<AppState>();
    if (n.unread) {
      try {
        await state.api.markNotificationRead(n.id);
        state.setUnread((state.unreadNotifications - 1).clamp(0, 99));
      } on ApiError {
        // vazhdon
      }
    }
    final link = n.deepLink;
    if (!mounted || link == null) return;
    final screen = _screenFor(link);
    if (screen == null) return;
    await Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => screen));
    if (mounted) await _load();
  }

  static Widget? _screenFor(String link) {
    final uri = Uri.tryParse(link);
    if (uri == null || uri.scheme != 'krejt') return null;
    final parts = [uri.host, ...uri.pathSegments].where((s) => s.isNotEmpty).toList();
    if (parts.length < 2) return null;
    final id = parts[1];
    switch (parts[0]) {
      case 'rides':
        return TrackingScreen(rideId: id);
      case 'orders':
        return OrderTrackingScreen(orderId: id);
      case 'parcels':
        return ParcelTrackingScreen(parcelId: id);
      case 'services':
        return ServiceTrackingScreen(requestId: id);
    }
    return null;
  }

  static IconData _icon(String category) {
    switch (category) {
      case 'rides':
        return Icons.directions_car_outlined;
      case 'orders':
        return Icons.receipt_long_outlined;
      case 'payments':
      case 'wallet':
        return Icons.account_balance_wallet_outlined;
      case 'security':
        return Icons.shield_outlined;
      case 'support':
        return Icons.support_agent_outlined;
    }
    return Icons.notifications_none;
  }

  @override
  Widget build(BuildContext context) {
    final unread = _items.where((n) => n.unread).length;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(context.t('notifications.title')),
        actions: [
          if (unread > 0)
            TextButton(
              onPressed: _readAll,
              child: Text(
                context.t('notifications.read_all'),
                style: const TextStyle(color: K.brand400, fontWeight: FontWeight.w600),
              ),
            ),
        ],
      ),
      body: SafeArea(
        child: _loading && _items.isEmpty
            ? const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 72, count: 4))
            : _items.isEmpty
            ? Padding(
                padding: const EdgeInsets.all(K.s5),
                child: _error != null
                    ? KError(
                        message: context.tError(_error!.messageKey),
                        retryLabel: context.t('common.retry'),
                        onRetry: _load,
                      )
                    : KEmpty(
                        title: context.t('notifications.empty'),
                        message: context.t('notifications.empty.hint'),
                        icon: Icons.notifications_none,
                      ),
              )
            : RefreshIndicator(
                onRefresh: _load,
                color: K.brand400,
                backgroundColor: K.surface2,
                child: ListView.builder(
                  padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
                  itemCount: _items.length,
                  itemBuilder: (_, i) {
                    final n = _items[i];
                    return Padding(
                      padding: const EdgeInsets.only(bottom: K.s2),
                      child: KCard(
                        onTap: () => _open(n),
                        highlight: n.unread,
                        padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Container(
                              width: 36,
                              height: 36,
                              alignment: Alignment.center,
                              decoration: BoxDecoration(
                                color: n.unread ? K.brand500.withValues(alpha: 0.14) : K.surface2,
                                borderRadius: BorderRadius.circular(K.rSm),
                              ),
                              child: Icon(
                                _icon(n.category),
                                size: 18,
                                color: n.unread ? K.brand400 : K.textDim,
                              ),
                            ),
                            const SizedBox(width: K.s3),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    context.t(n.titleKey, n.params),
                                    style: TextStyle(
                                      fontSize: 15,
                                      fontWeight: n.unread ? FontWeight.w700 : FontWeight.w600,
                                      color: K.text,
                                    ),
                                  ),
                                  const SizedBox(height: 2),
                                  Text(
                                    context.t(n.bodyKey, n.params),
                                    style: const TextStyle(
                                      fontSize: 13,
                                      color: K.textDim,
                                      height: 1.4,
                                    ),
                                  ),
                                  const SizedBox(height: K.s1),
                                  Text(
                                    _when(context, n.createdAt),
                                    style: const TextStyle(fontSize: 11, color: K.muted),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              ),
      ),
    );
  }

  /// Koha si e lexon njeriu: "tani", minuta, orë, pastaj data.
  static String _when(BuildContext context, DateTime at) {
    final d = DateTime.now().difference(at);
    if (d.inMinutes < 1) return context.t('time.now');
    if (d.inHours < 1) return context.t('time.minutes', {'n': '${d.inMinutes}'});
    if (d.inDays < 1) return context.t('time.hours', {'n': '${d.inHours}'});
    return '${at.day}.${at.month}.${at.year}';
  }
}
