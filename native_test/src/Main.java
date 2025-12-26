import com.google.common.base.Joiner;
import java.util.Arrays;

public class Main {
  public static void main(String[] args) {
    String joined = Joiner.on(", ").join(Arrays.asList("Hello", "Native", "Engine"));
    System.out.println(joined);
  }
}
