import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import 'addresses.dart';
import 'language_settings.dart';
import 'notifications.dart';
import 'profile.dart';
import 'sessions.dart';

/// Llogaria: një listë e vetme hyrjesh, secila hap një ekran të vetin.
/// Dalja nga llogaria kërkon konfirmim, sepse humbet sesionin e pajisjes (§53).
class AccountScreen extends StatelessWidget {
  const AccountScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final me = state.me;

    return SafeArea(
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
        children: [
          Text(
            context.t('account.title'),
            style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
          ),
          const SizedBox(height: K.s4),
          KCard(
            child: Row(
              children: [
                Container(
                  width: 52,
                  height: 52,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    gradient: K.gradient,
                    borderRadius: BorderRadius.circular(K.rFull),
                  ),
                  child: Text(
                    me?.initials ?? 'K',
                    style: const TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w700,
                      color: K.onBrand,
                    ),
                  ),
                ),
                const SizedBox(width: K.s4),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        me?.displayName ?? '—',
                        style: const TextStyle(
                          fontSize: 17,
                          fontWeight: FontWeight.w700,
                          color: K.text,
                        ),
                      ),
                      if (me?.phone != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 2),
                          child: Text(
                            me!.phone!,
                            style: const TextStyle(fontSize: 13, color: K.muted),
                          ),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: K.s5),
          _Entry(
            icon: Icons.person_outline,
            label: context.t('account.profile'),
            onTap: () => _open(context, const ProfileScreen()),
          ),
          _Entry(
            icon: Icons.place_outlined,
            label: context.t('account.addresses'),
            onTap: () => _open(context, const AddressesScreen()),
          ),
          _Entry(
            icon: Icons.notifications_none,
            label: context.t('account.notifications'),
            onTap: () => _open(context, const NotificationSettingsScreen()),
          ),
          _Entry(
            icon: Icons.devices_outlined,
            label: context.t('account.sessions'),
            onTap: () => _open(context, const SessionsScreen()),
          ),
          _Entry(
            icon: Icons.language,
            label: context.t('account.language'),
            value: KL10n.languageName(state.locale),
            onTap: () => _open(context, const LanguageSettingsScreen()),
          ),
          const SizedBox(height: K.s6),
          KOutlineButton(
            label: context.t('auth.logout'),
            icon: Icons.logout,
            danger: true,
            onPressed: () async {
              final ok = await confirmKSheet(
                context: context,
                title: context.t('auth.logout.confirm'),
                message: context.t('auth.logout.body'),
                confirmLabel: context.t('auth.logout'),
                cancelLabel: context.t('common.no'),
                destructive: true,
              );
              if (ok) await state.signOut();
            },
          ),
        ],
      ),
    );
  }

  void _open(BuildContext context, Widget screen) {
    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => screen));
  }
}

class _Entry extends StatelessWidget {
  const _Entry({required this.icon, required this.label, required this.onTap, this.value});

  final IconData icon;
  final String label;
  final String? value;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: K.s2),
    child: KCard(
      onTap: onTap,
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      child: SizedBox(
        height: K.minTap - K.s4,
        child: Row(
          children: [
            Icon(icon, size: 20, color: K.muted),
            const SizedBox(width: K.s3),
            Expanded(
              child: Text(
                label,
                style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
              ),
            ),
            if (value != null) Text(value!, style: const TextStyle(fontSize: 13, color: K.muted)),
            const SizedBox(width: K.s2),
            const Icon(Icons.chevron_right, size: 20, color: K.line2),
          ],
        ),
      ),
    ),
  );
}
