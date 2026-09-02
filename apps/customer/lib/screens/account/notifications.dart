import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Preferencat e njoftimeve. Kategoria e sigurisë mbetet e ndezur gjithmonë:
/// një kyçje e re nga një pajisje e panjohur duhet ta arrijë përdoruesin (§51).
class NotificationSettingsScreen extends StatefulWidget {
  const NotificationSettingsScreen({super.key});

  @override
  State<NotificationSettingsScreen> createState() => _NotificationSettingsScreenState();
}

class _NotificationSettingsScreenState extends State<NotificationSettingsScreen> {
  List<NotificationPreference> _items = const [];
  bool _loading = true;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final items = await context.read<AppState>().api.notificationPreferences();
      if (!mounted) return;
      setState(() {
        _items = items;
        _error = null;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  Future<void> _toggle(NotificationPreference p, {bool? push, bool? email, bool? sms}) async {
    if (p.locked) return;
    final updated = p.copyWith(push: push, email: email, sms: sms);
    setState(() {
      _items = _items.map((x) => x.category == p.category ? updated : x).toList();
    });
    final messenger = ScaffoldMessenger.of(context);
    try {
      await context.read<AppState>().api.updateNotificationPreference(updated);
    } on ApiError catch (e) {
      if (!mounted) return;
      // Kthejmë pamjen te gjendja e serverit; nuk lëmë çelës që duket i ndryshuar pa qenë.
      setState(() {
        _items = _items.map((x) => x.category == p.category ? p : x).toList();
      });
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('account.notifications'))),
      body: SafeArea(child: _body(context)),
    );
  }

  Widget _body(BuildContext context) {
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 96));
    }
    if (_error != null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error!.messageKey),
          retryLabel: context.t('common.retry'),
          onRetry: _load,
        ),
      );
    }
    return ListView(
      padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
      children: [
        for (final p in _items)
          Padding(
            padding: const EdgeInsets.only(bottom: K.s3),
            child: KCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          context.t('account.category.${p.category}'),
                          style: const TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w700,
                            color: K.text,
                          ),
                        ),
                      ),
                      if (p.locked) KBadge(context.t('common.settings'), tone: KTone.info),
                    ],
                  ),
                  if (p.locked)
                    Padding(
                      padding: const EdgeInsets.only(top: K.s1),
                      child: Text(
                        context.t('account.notifications.locked'),
                        style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
                      ),
                    ),
                  const SizedBox(height: K.s2),
                  _Toggle(
                    label: context.t('account.notifications.push'),
                    value: p.push,
                    enabled: !p.locked,
                    onChanged: (v) => _toggle(p, push: v),
                  ),
                  _Toggle(
                    label: context.t('account.notifications.email'),
                    value: p.email,
                    enabled: !p.locked,
                    onChanged: (v) => _toggle(p, email: v),
                  ),
                  _Toggle(
                    label: context.t('account.notifications.sms'),
                    value: p.sms,
                    enabled: !p.locked,
                    onChanged: (v) => _toggle(p, sms: v),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

class _Toggle extends StatelessWidget {
  const _Toggle({
    required this.label,
    required this.value,
    required this.enabled,
    required this.onChanged,
  });

  final String label;
  final bool value;
  final bool enabled;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) => SizedBox(
    height: K.minTap,
    child: Row(
      children: [
        Expanded(
          child: Text(label, style: TextStyle(fontSize: 14, color: enabled ? K.textDim : K.muted)),
        ),
        Switch(value: value, onChanged: enabled ? onChanged : null),
      ],
    ),
  );
}
