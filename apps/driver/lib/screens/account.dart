import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import '../state/work_state.dart';
import 'documents.dart';
import 'language_settings.dart';

/// Llogaria e shoferit. Dalja nga llogaria e nxjerr edhe nga puna, që dispeçeri të mos
/// vazhdojë t'i dërgojë kërkesa një pajisjeje që nuk përgjigjet më (§27).
class DriverAccountScreen extends StatelessWidget {
  const DriverAccountScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final driver = state.driver;
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
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  me?.displayName ?? '—',
                  style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: K.text),
                ),
                if (driver != null) ...[
                  const SizedBox(height: K.s2),
                  KRow(context.t('driver.offer.trip'), driver.vehicle),
                  KRow(context.t('account.profile'), driver.vehiclePlate),
                  if (driver.rating != null)
                    KRow(
                      context.t('ride.review.title'),
                      context.t('ride.driver.rating', {
                        'rating': driver.rating!.toStringAsFixed(1),
                      }),
                    ),
                ],
              ],
            ),
          ),
          const SizedBox(height: K.s5),
          _Entry(
            icon: Icons.description_outlined,
            label: context.t('driver.docs.title'),
            onTap: () =>
                Navigator.of(context)
                    .push(MaterialPageRoute<void>(builder: (_) => const DocumentsScreen())),
          ),
          _Entry(
            icon: Icons.language,
            label: context.t('account.language'),
            value: KL10n.languageName(state.locale),
            onTap: () =>
                Navigator.of(context)
                    .push(MaterialPageRoute<void>(builder: (_) => const DriverLanguageScreen())),
          ),
          const SizedBox(height: K.s6),
          KOutlineButton(
            label: context.t('auth.logout'),
            icon: Icons.logout,
            danger: true,
            onPressed: () async {
              final work = context.read<WorkState>();
              final ok = await confirmKSheet(
                context: context,
                title: context.t('auth.logout.confirm'),
                message: context.t('auth.logout.body'),
                confirmLabel: context.t('auth.logout'),
                cancelLabel: context.t('common.no'),
                destructive: true,
              );
              if (!ok) return;
              if (work.online) await work.goOffline();
              await state.signOut();
            },
          ),
        ],
      ),
    );
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
