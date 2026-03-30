import SwiftUI

struct VProgressBar: View {
    let progress: Double   // 0.0 – 1.0
    var height: CGFloat = 6
    var foreground: Color = Tokens.Colors.primary
    var background: Color = Tokens.Colors.border

    var body: some View {
        GeometryReader { geo in
            ZStack(alignment: .leading) {
                Capsule()
                    .fill(background)
                    .frame(height: height)

                Capsule()
                    .fill(foreground)
                    .frame(width: max(0, geo.size.width * progress), height: height)
                    .animation(.easeInOut(duration: 0.3), value: progress)
            }
        }
        .frame(height: height)
    }
}

#Preview {
    VStack(spacing: 12) {
        VProgressBar(progress: 0.0)
        VProgressBar(progress: 0.35)
        VProgressBar(progress: 0.7)
        VProgressBar(progress: 1.0)
    }
    .padding()
}
