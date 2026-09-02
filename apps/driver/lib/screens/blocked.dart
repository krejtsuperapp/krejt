import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Muri i mirëmbajtjes dhe i përditësimit të detyrueshëm. Nuk ka rrugë përreth tij:
/// serveri e vendos gjendjen, aplikacioni vetëm e shpjegon (§48).
class BlockedScreen extends StatelessWidget {
  const BlockedScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final maintenance = state.config.updateState == UpdateState.maintenance;
    final serverMessage = state.config.app?.maintenanceMessage;

    return Scaffold(
      backgroundColor: K.bg,
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(K.s6),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 72,
                  height: 72,
                  decoration: BoxDecoration(
                    color: maintenance ? K.warnBg : K.brand100,
                    borderRadius: BorderRadius.circular(K.rLg),
                  ),
                  child: Icon(
                    maintenance ? Icons.build_outlined : Icons.system_update_alt,
                    size: 32,
                    color: maintenance ? K.warn : K.brand400,
                  ),
                ),
                const SizedBox(height: K.s5),
                Text(
                  context.t(maintenance ? 'state.maintenance' : 'state.update.required'),
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
                ),
                const SizedBox(height: K.s2),
                Text(
                  serverMessage ??
                      context.t(
                        maintenance ? 'state.maintenance.body' : 'state.update.required.body',
                      ),
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 15, color: K.textDim, height: 1.45),
                ),
                const SizedBox(height: K.s6),
                // Te mirëmbajtja rikthimi ka kuptim; te përditësimi i detyrueshëm veprimi
                // ndodh në dyqanin e aplikacioneve, ndaj nuk shpikim buton që nuk bën gjë.
                if (maintenance)
                  KButton(label: context.t('common.retry'), onPressed: state.boot)
                else
                  Text(
                    context.t('state.update.now'),
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: K.brand400,
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
