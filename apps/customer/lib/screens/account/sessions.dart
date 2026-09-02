import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Pajisjet e kyçura. Heqja e një pajisjeje anulon menjëherë refresh token-in e saj (§53).
class SessionsScreen extends StatefulWidget {
  const SessionsScreen({super.key});

  @override
  State<SessionsScreen> createState() => _SessionsScreenState();
}

class _SessionsScreenState extends State<SessionsScreen> {
  List<DeviceSession> _items = const [];
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
      final items = await context.read<AppState>().api.sessions();
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

  Future<void> _revoke(DeviceSession s) async {
    final ok = await confirmKSheet(
      context: context,
      title: context.t('account.sessions.revoke.confirm'),
      message: s.deviceName ?? s.platform ?? '—',
      confirmLabel: context.t('account.sessions.revoke'),
      cancelLabel: context.t('common.no'),
      destructive: true,
    );
    if (!ok || !mounted) return;
    final state = context.read<AppState>();
    final messenger = ScaffoldMessenger.of(context);
    try {
      await state.api.revokeSession(s.id);
      if (s.current) {
        await state.signOut();
        return;
      }
      await _load();
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('account.sessions'))),
      body: SafeArea(child: _body(context)),
    );
  }

  Widget _body(BuildContext context) {
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 72));
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
        for (final s in _items)
          Padding(
            padding: const EdgeInsets.only(bottom: K.s2),
            child: KCard(
              child: Row(
                children: [
                  Icon(
                    s.platform == 'ios' ? Icons.phone_iphone : Icons.phone_android,
                    size: 20,
                    color: K.muted,
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Flexible(
                              child: Text(
                                s.deviceName ?? s.platform ?? '—',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontSize: 15,
                                  fontWeight: FontWeight.w600,
                                  color: K.text,
                                ),
                              ),
                            ),
                            if (s.current) ...[
                              const SizedBox(width: K.s2),
                              KBadge(context.t('account.sessions.current'), tone: KTone.ok),
                            ],
                          ],
                        ),
                        const SizedBox(height: 2),
                        Text(
                          '${s.lastSeenAt.day}.${s.lastSeenAt.month}.${s.lastSeenAt.year}'
                          '${s.ip == null ? '' : ' · ${s.ip}'}',
                          style: const TextStyle(fontSize: 12, color: K.muted),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.logout, color: K.muted),
                    tooltip: context.t('account.sessions.revoke'),
                    onPressed: () => _revoke(s),
                  ),
                ],
              ),
            ),
          ),
        const SizedBox(height: K.s4),
        KOutlineButton(
          label: context.t('auth.logout.all'),
          icon: Icons.logout,
          danger: true,
          onPressed: () async {
            final state = context.read<AppState>();
            final ok = await confirmKSheet(
              context: context,
              title: context.t('auth.logout.all'),
              message: context.t('auth.logout.body'),
              confirmLabel: context.t('auth.logout.all'),
              cancelLabel: context.t('common.no'),
              destructive: true,
            );
            if (ok) await state.api.logoutAll();
          },
        ),
      ],
    );
  }
}
