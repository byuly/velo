import SwiftUI

struct RootView: View {
    @State private var appState = AppState()
    @State private var navigationPath = NavigationPath()

    var body: some View {
        ZStack(alignment: .bottom) {
            Group {
                if !appState.isAuthenticated {
                    WelcomeView()
                } else if !appState.hasCompletedOnboarding {
                    OnboardingView()
                } else {
                    NavigationStack(path: $navigationPath) {
                        HomeView()
                    }
                    .onAppear {
                        if appState.activeSessionIntercept,
                           let active = appState.activeSession,
                           navigationPath.isEmpty {
                            navigationPath.append(NavDestination.session(active))
                        }
                    }
                }
            }
            .environment(appState)

            if let message = appState.toastMessage {
                VToast(message: message)
                    .padding(.bottom, 32)
                    .transition(.move(edge: .bottom).combined(with: .opacity))
                    .animation(.spring(response: 0.35), value: appState.toastMessage)
            }
        }
        .animation(.easeInOut(duration: 0.25), value: appState.isAuthenticated)
        .animation(.easeInOut(duration: 0.25), value: appState.hasCompletedOnboarding)
    }
}

#Preview {
    RootView()
}
