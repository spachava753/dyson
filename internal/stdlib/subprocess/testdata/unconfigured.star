# A subprocess module without a command runner must fail closed.

---
load("subprocess.star", "subprocess")

subprocess.run(["echo"], capture_output=True)  ### "subprocess.run: subprocess execution is not configured"

---
load("subprocess.star", "subprocess")

subprocess.getstatusoutput("echo hidden")  ### "subprocess.getstatusoutput: subprocess execution is not configured"
