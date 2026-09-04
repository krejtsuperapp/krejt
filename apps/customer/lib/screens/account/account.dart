import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../activity.dart';
import 'addresses.dart';
import 'language_settings.dart';
import 'legal.dart';
import 'notifications.dart';
import 'profile.dart';
import 'sessions.dart';

/// Llogaria: karta e profilit me foto, hyrjet e grupuara, dalja në fund.
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
            onTap: () => _open(context, const ProfileScreen()),
            child: Row(
              children: [
                KAvatar(url: me?.photoUrl, initials: me?.initials ?? 'K', size: 56),
                const SizedBox(width: K.s4),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        me?.displayName ?? '—',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
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
                const Icon(Icons.chevron_right, size: 20, color: K.line2),
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
            icon: Icons.history,
            label: context.t('activity.title'),
            onTap: () => _open(context, const ActivityScreen()),
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
          _Entry(
            icon: Icons.description_outlined,
            label: context.t('account.legal'),
            onTap: () => _open(context, const LegalScreen()),
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
          const SizedBox(height: K.s6),
          const Center(
            child: KWordmark(size: 18, animate: false, color: K.muted, barColor: K.line2),
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
            Container(
              width: 34,
              height: 34,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: K.surface2,
                borderRadius: BorderRadius.circular(K.rSm),
              ),
              child: Icon(icon, size: 18, color: K.textDim),
            ),
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
