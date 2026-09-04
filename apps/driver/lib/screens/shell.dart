import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'account.dart';
import 'earnings.dart';
import 'services.dart';
import 'work.dart';

/// Katër destinacione: puna, shërbimet, fitimet, llogaria. Puna rri e para dhe mbetet e para,
/// sepse aty ndodh gjithçka gjatë turnit; shërbimet janë punë e llojit tjetër, me ritëm tjetër.
class DriverShell extends StatefulWidget {
  const DriverShell({super.key});

  @override
  State<DriverShell> createState() => _DriverShellState();
}

class _DriverShellState extends State<DriverShell> {
  int _index = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<AppState>().refreshDriver();
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      body: IndexedStack(
        index: _index,
        children: const [WorkScreen(), ServicesScreen(), EarningsScreen(), DriverAccountScreen()],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: [
          NavigationDestination(
            icon: const Icon(Icons.local_taxi_outlined),
            selectedIcon: const Icon(Icons.local_taxi),
            label: context.t('driver.nav.work'),
          ),
          NavigationDestination(
            icon: const Icon(Icons.handyman_outlined),
            selectedIcon: const Icon(Icons.handyman),
            label: context.t('provider.nav'),
          ),
          NavigationDestination(
            icon: const Icon(Icons.payments_outlined),
            selectedIcon: const Icon(Icons.payments),
            label: context.t('driver.nav.earnings'),
          ),
          NavigationDestination(
            icon: const Icon(Icons.person_outline),
            selectedIcon: const Icon(Icons.person),
            label: context.t('driver.nav.account'),
          ),
        ],
      ),
    );
  }
}
